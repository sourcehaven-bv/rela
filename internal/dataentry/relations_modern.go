package dataentry

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Warning is a type alias for entity.Warning so that handlers
// in this package can write `dataentry.Warning` without importing the
// entitymanager package at every call site. Behavior is identical.
type Warning = entity.Warning

// validateRelationsModern runs the validation phase of the modern
// reconciler without performing any writes. It returns:
//
//   - warnings: soft-condition findings (DEC-HWZHA write-with-warnings).
//     Edges flagged by warnings will still be written by applyRelationsModern.
//   - err: a hard 422 (structural impossibility) or 400 (caller bug). On err
//     the caller MUST NOT proceed to the entity-update or write phases.
//
// This split lets handleV1UpdateEntity validate relations BEFORE the
// entity is updated, so a structural relation problem doesn't leave
// the entity half-written.
func (a *App) validateRelationsModern(
	ctx context.Context, entityID string, pathEntityType string, desired map[string]V1RelationsUpdate,
) ([]Warning, error) {
	if len(desired) == 0 {
		return nil, nil
	}
	meta := a.State().Meta
	var warnings []Warning

	// Self-loop shape_conflict detection: a body that references the
	// path entity under BOTH a canonical name AND its inverse for the
	// same canonical relation refers to the same physical edge twice.
	// Skipped for symmetric relations (whose inverse equals canonical
	// and which have no preferred direction).
	if err := detectSelfLoopShapeConflict(meta, entityID, desired); err != nil {
		return nil, err
	}

	for bodyKey, upd := range desired {
		if !upd.DataPresent {
			return nil, &wireError{
				Code:   "data_required",
				Path:   "/relations/" + jsonPointerEscape(bodyKey) + "/data",
				Detail: "`data` field is required when a relation wrapper appears",
			}
		}

		canonical, incoming, ok := resolveDirection(meta, bodyKey)
		if !ok {
			return nil, &structuralError{
				Code:   "unknown_relation_type",
				Path:   "/relations/" + jsonPointerEscape(bodyKey),
				Detail: fmt.Sprintf("relation type %q is not defined in the metamodel", bodyKey),
			}
		}
		relDef := meta.Relations[canonical]

		// Path-entity-side type allowlist:
		//   - outgoing: path entity is source, check relDef.From
		//   - incoming: path entity is target, check relDef.To
		allowedPathTypes := relDef.From
		warningCode := "source_type_not_allowed"
		if incoming {
			allowedPathTypes = relDef.To
			warningCode = "target_type_not_allowed"
		}
		if !containsString(allowedPathTypes, pathEntityType) {
			warnings = append(warnings, Warning{
				Code:      warningCode,
				Path:      "/relations/" + jsonPointerEscape(bodyKey),
				Detail:    fmt.Sprintf("relation %q does not declare %q as an allowed %s type", canonical, pathEntityType, sideLabel(incoming, true)),
				Direction: directionLabel(incoming),
			})
		}

		for i, ref := range upd.Data {
			edgePath := fmt.Sprintf("/relations/%s/data/%d", jsonPointerEscape(bodyKey), i)

			// Content on a non-content-bearing relation type is a
			// structural impossibility — the file format can't hold
			// a body for that type.
			if ref.Content != nil && !relDef.Content {
				return nil, &structuralError{
					Code:   "content_not_supported",
					Path:   edgePath + "/content",
					Detail: fmt.Sprintf("relation type %q does not support per-edge content", canonical),
				}
			}

			// Soft conditions surfaced as warnings. The peer is whichever
			// side the path entity is NOT on.
			ws := a.collectEdgeWarnings(ctx, canonical, &relDef, ref, edgePath, incoming)
			warnings = append(warnings, ws...)
		}
	}
	return warnings, nil
}

// directionLabel returns the JSON-friendly string for the direction
// flag. Matches the Warning.Direction field contract from TKT-GFQK.
func directionLabel(incoming bool) string {
	if incoming {
		return "incoming"
	}
	return "outgoing"
}

// sideLabel produces a human-readable side description for warning
// messages ("source" / "target"). pathSide=true asks for the side the
// path entity is on; pathSide=false asks for the peer side.
func sideLabel(incoming, pathSide bool) string {
	if incoming == pathSide {
		return "target"
	}
	return "source"
}

// collectEdgeWarnings runs the soft-condition checks for a single
// resource identifier. None of these block the write.
//
// `incoming` flips the side of the type-allowlist checks: when true,
// the `ref` is the SOURCE side of the canonical edge (the path entity
// is the target). Warning codes stay the same so client de-dup by
// code keeps working; the `Direction` field disambiguates.
func (a *App) collectEdgeWarnings(
	ctx context.Context, relType string, relDef *metamodel.RelationDef,
	ref V1ResourceIdentifier, edgePath string, incoming bool,
) []Warning {
	var warnings []Warning
	meta := a.State().Meta
	direction := directionLabel(incoming)

	peer, err := a.store.GetEntity(ctx, ref.ID)
	if err != nil {
		warnings = append(warnings, Warning{
			Code:      "target_not_found",
			Path:      edgePath + "/id",
			Detail:    fmt.Sprintf("peer entity %q does not exist; the edge will be created but reference a missing peer", ref.ID),
			Direction: direction,
		})
	} else {
		if peer.Type != ref.Type {
			warnings = append(warnings, Warning{
				Code:      "target_type_mismatch",
				Path:      edgePath + "/type",
				Detail:    fmt.Sprintf("expected peer type %q, but %q is of type %q", ref.Type, ref.ID, peer.Type),
				Direction: direction,
			})
		}
		// Peer-side type allowlist: when path entity is source
		// (outgoing), the peer is target and must be in relDef.To;
		// when path entity is target (incoming), the peer is source
		// and must be in relDef.From.
		peerSideAllowed := relDef.To
		if incoming {
			peerSideAllowed = relDef.From
		}
		if !containsString(peerSideAllowed, peer.Type) {
			warnings = append(warnings, Warning{
				Code:      "target_type_not_allowed",
				Path:      edgePath,
				Detail:    fmt.Sprintf("relation %q does not declare %q as an allowed %s type", relType, peer.Type, sideLabel(incoming, false)),
				Direction: direction,
			})
		}
	}

	// Closed-schema check on meta keys.
	for k := range ref.Meta {
		if _, known := relDef.Properties[k]; !known {
			warnings = append(warnings, Warning{
				Code:      "unknown_meta_key",
				Path:      edgePath + "/meta/" + jsonPointerEscape(k),
				Detail:    fmt.Sprintf("relation type %q does not declare meta property %q", relType, k),
				Direction: direction,
			})
		}
	}
	for _, k := range ref.MetaUnset {
		if _, known := relDef.Properties[k]; !known {
			warnings = append(warnings, Warning{
				Code:      "unknown_meta_key",
				Path:      edgePath + "/meta_unset",
				Detail:    fmt.Sprintf("relation type %q does not declare meta property %q", relType, k),
				Direction: direction,
			})
		}
	}

	// Per-property type validation for declared keys with provided values.
	for k, v := range ref.Meta {
		propDef, known := relDef.Properties[k]
		if !known {
			continue // already warned above
		}
		if err := meta.ValidatePropertyValue(k, &propDef, v); err != nil {
			warnings = append(warnings, Warning{
				Code:      "meta_type_mismatch",
				Path:      edgePath + "/meta/" + jsonPointerEscape(k),
				Detail:    err.Error(),
				Direction: direction,
			})
		}
	}

	return warnings
}

// applyRelationsModern performs the diff and write phase of the modern
// reconciler. Validation should have run already via
// validateRelationsModern; this function does NOT re-validate. It only
// surfaces additional warnings that depend on post-merge state (e.g.
// required-meta-unset) and store-write errors.
//
// Edges flagged by the validation phase as "target missing" or
// "target type mismatch" are written directly through the store
// rather than the EntityManager — the EntityManager's CreateRelation
// rejects writes whose target doesn't exist, but DEC-HWZHA's policy is
// to permit the write with a warning. The store does not check target
// existence, so the direct write succeeds and `analyze_*` flags it on
// the next run.
//
// Returns warnings collected during the apply phase plus any error
// that prevented further writes. On a write-loop error, the relations
// already written stay written — the caller treats this as the
// documented atomicity gap.
func (a *App) applyRelationsModern(
	ctx context.Context, entityID string, desired map[string]V1RelationsUpdate,
) ([]Warning, error) {
	if len(desired) == 0 {
		return nil, nil
	}
	meta := a.State().Meta
	em := a.entityManager
	var warnings []Warning

	for bodyKey, upd := range desired {
		canonical, incoming, ok := resolveDirection(meta, bodyKey)
		if !ok {
			// validateRelationsModern already screened this; defensive.
			return warnings, &structuralError{
				Code:   "unknown_relation_type",
				Path:   "/relations/" + jsonPointerEscape(bodyKey),
				Detail: fmt.Sprintf("relation type %q is not defined in the metamodel", bodyKey),
			}
		}
		relDef := meta.Relations[canonical]
		direction := directionLabel(incoming)

		desiredByID := make(map[string]V1ResourceIdentifier, len(upd.Data))
		for _, ref := range upd.Data {
			desiredByID[ref.ID] = ref
		}

		current := a.currentEdgesByPeer(entityID, canonical, incoming)

		// Adds and upserts.
		for _, ref := range upd.Data {
			finalProps, finalContent, contentSet := mergeEdgeMeta(current[ref.ID], ref)

			ws := requiredMetaWarnings(canonical, &relDef, ref, finalProps,
				fmt.Sprintf("/relations/%s/data", jsonPointerEscape(bodyKey)), direction)
			warnings = append(warnings, ws...)

			from, to := edgeEndpoints(entityID, ref.ID, incoming)

			existing, exists := current[ref.ID]
			if exists {
				if isEdgeNoOp(existing, finalProps, finalContent, contentSet, ref) {
					continue // value-based no-op suppression
				}
				if err := a.writeUpdateRelation(ctx, from, to, canonical, ref); err != nil {
					return warnings, err
				}
			} else {
				if err := a.writeCreateRelation(ctx, from, to, canonical, ref, finalProps, finalContent); err != nil {
					return warnings, err
				}
			}
		}

		// Deletes: every current edge not in the desired set.
		for peerID := range current {
			if _, kept := desiredByID[peerID]; kept {
				continue
			}
			from, to := edgeEndpoints(entityID, peerID, incoming)
			if err := em.DeleteRelation(ctx, from, canonical, to); err != nil {
				return warnings, &relationError{
					RelType: canonical, Target: peerID, Op: "delete",
					Reason: "delete_failed", Err: err,
				}
			}
		}
	}
	return warnings, nil
}

// writeCreateRelation creates a new relation. It first tries the
// EntityManager (preserving validation + automation paths for the
// happy case); on a target-not-found error it falls back to a direct
// store write so DEC-HWZHA's "soft conditions are warnings, not
// rejections" policy holds.
//
// `from` and `to` are pre-resolved by the caller via edgeEndpoints —
// this function does not consult direction. `ref.ID` is the peer ID
// (which is `to` for outgoing edges and `from` for incoming).
func (a *App) writeCreateRelation(
	ctx context.Context, from, to, relType string, ref V1ResourceIdentifier,
	finalProps map[string]interface{}, finalContent string,
) error {
	opts := entity.RelationOptions{
		Properties: finalProps,
		Content:    ref.Content,
	}
	_, err := a.entityManager.CreateRelation(ctx, from, relType, to, opts)
	if err == nil {
		return nil
	}
	if !isSoftCondition(err) {
		return &relationError{
			RelType: relType, Target: ref.ID, Op: "create",
			Reason: "create_failed", Err: err,
		}
	}
	// Soft condition (e.g., target missing): write directly through
	// the store, skipping the workspace's pre-write validation.
	data := &store.RelationData{Properties: finalProps, Content: finalContent}
	if _, sErr := a.store.CreateRelation(ctx, from, relType, to, data); sErr != nil {
		return &relationError{
			RelType: relType, Target: ref.ID, Op: "create",
			Reason: "create_failed", Err: sErr,
		}
	}
	return nil
}

// writeUpdateRelation updates an existing relation, preferring the
// EntityManager path. Falls back to a direct store write on soft
// conditions, mirroring writeCreateRelation.
//
// `from` and `to` are pre-resolved by the caller via edgeEndpoints.
func (a *App) writeUpdateRelation(
	ctx context.Context, from, to, relType string, ref V1ResourceIdentifier,
) error {
	opts := entity.RelationOptions{
		Properties: ref.Meta,
		MetaUnset:  ref.MetaUnset,
		Content:    ref.Content,
	}
	_, err := a.entityManager.UpdateRelation(ctx, from, relType, to, opts)
	if err == nil {
		return nil
	}
	if !isSoftCondition(err) {
		return &relationError{
			RelType: relType, Target: ref.ID, Op: "update",
			Reason: "update_failed", Err: err,
		}
	}
	// Soft condition: rebuild the post-merge state and write directly.
	current, _ := a.store.GetRelation(ctx, from, relType, to)
	finalProps, finalContent, _ := mergeEdgeMeta(current, ref)
	data := store.RelationData{Properties: finalProps, Content: finalContent}
	if _, sErr := a.store.UpdateRelation(ctx, from, relType, to, data); sErr != nil {
		return &relationError{
			RelType: relType, Target: ref.ID, Op: "update",
			Reason: "update_failed", Err: sErr,
		}
	}
	return nil
}

// isSoftCondition returns true when the error from EntityManager
// indicates a DEC-HWZHA "soft" condition that should be treated as a
// warning rather than blocking the write. The current workspace
// implementation surfaces these as plain fmt.Errorf strings; we match
// on substrings, which is fragile but acceptable for the current
// implementation surface.
func isSoftCondition(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "target entity not found"):
		return true
	case strings.Contains(msg, "source entity not found"):
		return true
	case strings.Contains(msg, "invalid relation:"):
		// metamodel.ValidateRelation rejects type-allowlist failures.
		return true
	}
	return false
}

// mergeEdgeMeta computes the post-merge (properties, content) tuple
// for an edge, given the existing relation (or nil for new edges) and
// the desired ref. contentSet reports whether ref.Content was non-nil
// (so the caller can distinguish "leave alone" from "set to value").
func mergeEdgeMeta(existing *entity.Relation, ref V1ResourceIdentifier) (
	props map[string]interface{}, content string, contentSet bool,
) {
	props = make(map[string]interface{})
	if existing != nil {
		for k, v := range existing.Properties {
			props[k] = v
		}
		content = existing.Content
	}
	for k, v := range ref.Meta {
		props[k] = v
	}
	for _, k := range ref.MetaUnset {
		delete(props, k)
	}
	if ref.Content != nil {
		content = *ref.Content
		contentSet = true
	}
	return props, content, contentSet
}

// isEdgeNoOp returns true when the post-merge state of an edge equals
// the existing edge byte-for-byte. Auto-save's primary path hits this:
// re-PATCHing a form that hasn't changed performs zero writes.
func isEdgeNoOp(
	existing *entity.Relation, finalProps map[string]interface{},
	finalContent string, contentSet bool, ref V1ResourceIdentifier,
) bool {
	// If the request has no per-edge upsert fields at all and the
	// edge already exists, it's trivially a no-op. (Strict subset of
	// the value-based check below; kept as a fast path.)
	if ref.Meta == nil && ref.MetaUnset == nil && ref.Content == nil {
		return true
	}
	if !mapsEqual(existing.Properties, finalProps) {
		return false
	}
	if contentSet && existing.Content != finalContent {
		return false
	}
	return true
}

// mapsEqual is a small wrapper around reflect.DeepEqual that treats
// nil and empty maps as equal. (Go's reflect.DeepEqual considers
// nil != empty map, which would produce false-positive writes on
// edges whose existing properties are nil and whose final state is
// an empty map after merge.)
func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

// requiredMetaWarnings returns warnings for declared-required meta
// keys that are absent from the post-merge state.
//
// `direction` is the direction label ("outgoing" or "incoming") to
// stamp on the emitted Warning so UIs can disambiguate same-edge
// warnings without parsing paths. Per-edge meta is currently
// relation-type-scoped (not per-direction), so direction does not
// affect WHICH keys are required — it only labels the output.
func requiredMetaWarnings(
	relType string, relDef *metamodel.RelationDef, ref V1ResourceIdentifier,
	finalProps map[string]interface{}, dataPath, direction string,
) []Warning {
	var ws []Warning
	for k, propDef := range relDef.Properties {
		if !propDef.Required {
			continue
		}
		if _, ok := finalProps[k]; !ok {
			ws = append(ws, Warning{
				Code:      "required_meta_unset",
				Path:      fmt.Sprintf("%s[id=%s]/meta/%s", dataPath, jsonPointerEscape(ref.ID), jsonPointerEscape(k)),
				Detail:    fmt.Sprintf("relation type %q requires meta property %q", relType, k),
				Direction: direction,
			})
		}
	}
	return ws
}

// structuralError is a typed hard-422 error from the modern reconciler:
// the request describes a state the storage layer can't represent. The
// HTTP handler maps it to 422 with the carried code.
type structuralError struct {
	Code   string
	Path   string
	Detail string
}

func (e *structuralError) Error() string {
	return fmt.Sprintf("%s: %s (path: %s)", e.Code, e.Detail, e.Path)
}

// asStructuralError extracts a *structuralError if err is or wraps one.
func asStructuralError(err error) (*structuralError, bool) {
	var se *structuralError
	if errors.As(err, &se) {
		return se, true
	}
	return nil, false
}
