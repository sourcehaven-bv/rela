package dataentry

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Warning is a non-blocking finding surfaced to the caller alongside
// a successful PATCH response. Code values match the corresponding
// `analyze_*` finding codes so the UI can de-duplicate against
// analyze runs. Path is an RFC 6901 JSON Pointer.
type Warning struct {
	Code   string `json:"code"`
	Path   string `json:"path,omitempty"`
	Detail string `json:"detail,omitempty"`
}

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
	ctx context.Context, _ string, sourceType string, desired map[string]V1RelationsUpdate,
) ([]Warning, error) {
	if len(desired) == 0 {
		return nil, nil
	}
	meta := a.State().Meta
	var warnings []Warning

	for relType, upd := range desired {
		if !upd.DataPresent {
			return nil, &wireError{
				Code:   "data_required",
				Path:   "/relations/" + jsonPointerEscape(relType) + "/data",
				Detail: "`data` field is required when a relation wrapper appears",
			}
		}

		relDef, ok := meta.Relations[relType]
		if !ok {
			return nil, &structuralError{
				Code:   "unknown_relation_type",
				Path:   "/relations/" + jsonPointerEscape(relType),
				Detail: fmt.Sprintf("relation type %q is not defined in the metamodel", relType),
			}
		}

		// Source-type allowlist: a soft warning per DEC-HWZHA. The
		// edge can still be written; analyze flags it.
		if !containsString(relDef.From, sourceType) {
			warnings = append(warnings, Warning{
				Code:   "source_type_not_allowed",
				Path:   "/relations/" + jsonPointerEscape(relType),
				Detail: fmt.Sprintf("relation %q does not declare %q as an allowed source type", relType, sourceType),
			})
		}

		for i, ref := range upd.Data {
			edgePath := fmt.Sprintf("/relations/%s/data/%d", jsonPointerEscape(relType), i)

			// Content on a non-content-bearing relation type is a
			// structural impossibility — the file format can't hold
			// a body for that type.
			if ref.Content != nil && !relDef.Content {
				return nil, &structuralError{
					Code:   "content_not_supported",
					Path:   edgePath + "/content",
					Detail: fmt.Sprintf("relation type %q does not support per-edge content", relType),
				}
			}

			// Soft conditions surfaced as warnings:
			ws := a.collectEdgeWarnings(ctx, relType, &relDef, ref, edgePath)
			warnings = append(warnings, ws...)
		}
	}
	return warnings, nil
}

// collectEdgeWarnings runs the soft-condition checks for a single
// resource identifier. None of these block the write.
func (a *App) collectEdgeWarnings(
	ctx context.Context, relType string, relDef *metamodel.RelationDef,
	ref V1ResourceIdentifier, edgePath string,
) []Warning {
	var warnings []Warning
	meta := a.State().Meta

	target, err := a.store.GetEntity(ctx, ref.ID)
	if err != nil {
		warnings = append(warnings, Warning{
			Code:   "target_not_found",
			Path:   edgePath + "/id",
			Detail: fmt.Sprintf("target entity %q does not exist; the edge will be created but reference a missing target", ref.ID),
		})
	} else {
		if target.Type != ref.Type {
			warnings = append(warnings, Warning{
				Code:   "target_type_mismatch",
				Path:   edgePath + "/type",
				Detail: fmt.Sprintf("expected target type %q, but %q is of type %q", ref.Type, ref.ID, target.Type),
			})
		}
		if !containsString(relDef.To, target.Type) {
			warnings = append(warnings, Warning{
				Code:   "target_type_not_allowed",
				Path:   edgePath,
				Detail: fmt.Sprintf("relation %q does not declare %q as an allowed target type", relType, target.Type),
			})
		}
	}

	// Closed-schema check on meta keys.
	for k := range ref.Meta {
		if _, known := relDef.Properties[k]; !known {
			warnings = append(warnings, Warning{
				Code:   "unknown_meta_key",
				Path:   edgePath + "/meta/" + jsonPointerEscape(k),
				Detail: fmt.Sprintf("relation type %q does not declare meta property %q", relType, k),
			})
		}
	}
	for _, k := range ref.MetaUnset {
		if _, known := relDef.Properties[k]; !known {
			warnings = append(warnings, Warning{
				Code:   "unknown_meta_key",
				Path:   edgePath + "/meta_unset",
				Detail: fmt.Sprintf("relation type %q does not declare meta property %q", relType, k),
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
				Code:   "meta_type_mismatch",
				Path:   edgePath + "/meta/" + jsonPointerEscape(k),
				Detail: err.Error(),
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

	for relType, upd := range desired {
		relDef := meta.Relations[relType] // already verified in validateRelationsModern

		desiredByID := make(map[string]V1ResourceIdentifier, len(upd.Data))
		for _, ref := range upd.Data {
			desiredByID[ref.ID] = ref
		}

		current := map[string]*entity.Relation{}
		for _, edge := range a.outgoingRelations(entityID) {
			if edge.Type == relType {
				current[edge.To] = edge
			}
		}

		// Adds and upserts.
		for _, ref := range upd.Data {
			finalProps, finalContent, contentSet := mergeEdgeMeta(current[ref.ID], ref)

			ws := requiredMetaWarnings(relType, &relDef, ref, finalProps,
				fmt.Sprintf("/relations/%s/data", jsonPointerEscape(relType)))
			warnings = append(warnings, ws...)

			existing, exists := current[ref.ID]
			if exists {
				if isEdgeNoOp(existing, finalProps, finalContent, contentSet, ref) {
					continue // value-based no-op suppression
				}
				if err := a.writeUpdateRelation(ctx, entityID, relType, ref); err != nil {
					return warnings, err
				}
			} else {
				if err := a.writeCreateRelation(ctx, entityID, relType, ref, finalProps, finalContent); err != nil {
					return warnings, err
				}
			}
		}

		// Deletes: every current edge not in the desired set.
		for targetID := range current {
			if _, kept := desiredByID[targetID]; kept {
				continue
			}
			if err := em.DeleteRelation(ctx, entityID, relType, targetID); err != nil {
				return warnings, &relationError{
					RelType: relType, Target: targetID, Op: "delete",
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
func (a *App) writeCreateRelation(
	ctx context.Context, from, relType string, ref V1ResourceIdentifier,
	finalProps map[string]interface{}, finalContent string,
) error {
	opts := entitymanager.RelationOptions{
		Properties: finalProps,
		Content:    ref.Content,
	}
	_, err := a.entityManager.CreateRelation(ctx, from, relType, ref.ID, opts)
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
	if _, sErr := a.store.CreateRelation(ctx, from, relType, ref.ID, data); sErr != nil {
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
func (a *App) writeUpdateRelation(
	ctx context.Context, from, relType string, ref V1ResourceIdentifier,
) error {
	opts := entitymanager.RelationOptions{
		Properties: ref.Meta,
		MetaUnset:  ref.MetaUnset,
		Content:    ref.Content,
	}
	_, err := a.entityManager.UpdateRelation(ctx, from, relType, ref.ID, opts)
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
	current, _ := a.store.GetRelation(ctx, from, relType, ref.ID)
	finalProps, finalContent, _ := mergeEdgeMeta(current, ref)
	data := store.RelationData{Properties: finalProps, Content: finalContent}
	if _, sErr := a.store.UpdateRelation(ctx, from, relType, ref.ID, data); sErr != nil {
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
func requiredMetaWarnings(
	relType string, relDef *metamodel.RelationDef, ref V1ResourceIdentifier,
	finalProps map[string]interface{}, dataPath string,
) []Warning {
	var ws []Warning
	for k, propDef := range relDef.Properties {
		if !propDef.Required {
			continue
		}
		if _, ok := finalProps[k]; !ok {
			ws = append(ws, Warning{
				Code:   "required_meta_unset",
				Path:   fmt.Sprintf("%s[id=%s]/meta/%s", dataPath, jsonPointerEscape(ref.ID), jsonPointerEscape(k)),
				Detail: fmt.Sprintf("relation type %q requires meta property %q", relType, k),
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
