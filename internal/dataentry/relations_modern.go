package dataentry

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
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
func (h *writeHandler) validateRelationsModern(
	ctx context.Context, entityID string, pathEntityType string, desired map[string]v1.RelationsUpdate,
) ([]Warning, error) {
	if len(desired) == 0 {
		return nil, nil
	}
	meta := h.schema().Meta
	var warnings []Warning

	// Self-loop shape_conflict detection: a body that references the
	// path entity under BOTH a canonical name AND its inverse for the
	// same canonical relation refers to the same physical edge twice.
	// Skipped for symmetric relations (whose inverse equals canonical
	// and which have no preferred direction).
	if err := detectSelfLoopShapeConflict(meta, entityID, desired); err != nil {
		return nil, err
	}

	// The structural checks run first, so a request they reject pays no
	// read. The edges' soft checks wait for one gated batch over every peer.
	type edgeCheck struct {
		relType  string
		relDef   metamodel.RelationDef
		ref      v1.ResourceIdentifier
		path     string
		incoming bool
		peer     entity.Address // the ref's id, parsed; see peerAddress
	}
	var edges []edgeCheck

	for bodyKey, upd := range desired {
		if !upd.DataPresent && !upd.Delta {
			return nil, &v1.WireError{
				Code:   "data_required",
				Path:   "/relations/" + v1.JSONPointerEscape(bodyKey) + "/data",
				Detail: "`data` field is required when a relation wrapper appears",
			}
		}

		canonical, incoming, ok := resolveDirection(meta, bodyKey)
		if !ok {
			return nil, &structuralError{
				Code:   "unknown_relation_type",
				Path:   "/relations/" + v1.JSONPointerEscape(bodyKey),
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
				Path:      "/relations/" + v1.JSONPointerEscape(bodyKey),
				Detail:    fmt.Sprintf("relation %q does not declare %q as an allowed %s type", canonical, pathEntityType, sideLabel(incoming, true)),
				Direction: directionLabel(incoming),
			})
		}

		for i, ref := range upd.Upserts() {
			edgePath := fmt.Sprintf("/relations/%s%s/%d", v1.JSONPointerEscape(bodyKey), upsertKey(upd), i)

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

			// Wire-format violation for managed order properties: only
			// finite numeric values are acceptable on _order_out / _order_in
			// because the engine's sort/midpoint math depends on them.
			if err := validateManagedOrderMeta(canonical, ref, edgePath, &relDef); err != nil {
				return nil, err
			}

			edges = append(edges, edgeCheck{canonical, relDef, ref, edgePath, incoming, peerAddress(ref.ID, incoming)})
		}
	}

	// Soft conditions surfaced as warnings. The peer is whichever side the
	// path entity is NOT on. Every peer is gated in one batch, so the cost
	// does not grow with the number of edges.
	ids := make([]string, len(edges))
	for i, e := range edges {
		ids[i] = e.peer.ID()
	}
	peerTypes, err := h.visible.readableTypes(ctx, ids)
	if err != nil {
		return nil, &gateFaultError{err: err}
	}
	for _, e := range edges {
		ws := h.collectEdgeWarnings(e.relType, &e.relDef, e.ref, peerTypes[e.peer.ID()], e.path, e.incoming)
		warnings = append(warnings, ws...)
	}

	// An incoming content edge names a face of its peer, and remove may
	// name only edges that exist; planning reports either failure. The
	// tail is unused: it picks outgoing edges only.
	for bodyKey, upd := range desired {
		canonical, incoming, _ := resolveDirection(meta, bodyKey)
		if !incoming || !meta.Relations[canonical].Scope.IsContent() {
			continue
		}
		if _, err := h.planEdges(ctx, entityID, entity.ImplicitFace, canonical, true, upd,
			"/relations/"+v1.JSONPointerEscape(bodyKey)); err != nil {
			return nil, err
		}
	}
	return warnings, nil
}

// upsertKey is the JSON pointer segment of a wrapper's upserted edges.
func upsertKey(upd v1.RelationsUpdate) string {
	if upd.Delta {
		return "/add"
	}
	return "/data"
}

// peerAddress parses a relation body's peer id. Only an incoming edge's
// peer may carry a face: it is the edge's source, and a content-scoped edge
// belongs to one face of it. An outgoing peer is the target, which is always
// the whole entity, so its id is taken as written; an unparseable one
// likewise, and the peer lookup then reports it missing.
func peerAddress(id string, incoming bool) entity.Address {
	if incoming {
		if addr, err := entity.ParseAddress(id); err == nil {
			return addr
		}
	}
	return entity.BareAddress(id)
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
//
// peerType is the peer's stored type when the principal may read some face
// of it, and "" otherwise ([visibleReader.readableTypes]). The peer is named
// by bare id, so it names an entity, not a face. A peer the caller may not
// read gets the same warning as an absent one, so neither its existence nor
// its type leaks through the warning codes.
func (h *writeHandler) collectEdgeWarnings(
	relType string, relDef *metamodel.RelationDef,
	ref v1.ResourceIdentifier, peerType, edgePath string, incoming bool,
) []Warning {
	var warnings []Warning
	meta := h.schema().Meta
	direction := directionLabel(incoming)

	if peerType == "" {
		warnings = append(warnings, Warning{
			Code:      "target_not_found",
			Path:      edgePath + "/id",
			Detail:    fmt.Sprintf("peer entity %q does not exist; the edge will be created but reference a missing peer", ref.ID),
			Direction: direction,
		})
	} else {
		if peerType != ref.Type {
			warnings = append(warnings, Warning{
				Code:      "target_type_mismatch",
				Path:      edgePath + "/type",
				Detail:    fmt.Sprintf("expected peer type %q, but %q is of type %q", ref.Type, ref.ID, peerType),
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
		if !containsString(peerSideAllowed, peerType) {
			warnings = append(warnings, Warning{
				Code:      "target_type_not_allowed",
				Path:      edgePath,
				Detail:    fmt.Sprintf("relation %q does not declare %q as an allowed %s type", relType, peerType, sideLabel(incoming, false)),
				Direction: direction,
			})
		}
	}

	// Managed order properties are recognized when the relation type is
	// orderable on the corresponding side. They are not declared in
	// relDef.Properties — the engine reserves them.
	isManagedOrderProperty := func(name string) bool {
		return (name == metamodel.OrderPropertyOut && relDef.OutgoingOrderProperty() != "") ||
			(name == metamodel.OrderPropertyIn && relDef.IncomingOrderProperty() != "")
	}

	// Closed-schema check on meta keys.
	for k := range ref.Meta {
		if isManagedOrderProperty(k) {
			continue
		}
		if _, known := relDef.Properties[k]; !known {
			warnings = append(warnings, Warning{
				Code:      "unknown_meta_key",
				Path:      edgePath + "/meta/" + v1.JSONPointerEscape(k),
				Detail:    fmt.Sprintf("relation type %q does not declare meta property %q", relType, k),
				Direction: direction,
			})
		}
	}
	for _, k := range ref.MetaUnset {
		if isManagedOrderProperty(k) {
			continue
		}
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
				Path:      edgePath + "/meta/" + v1.JSONPointerEscape(k),
				Detail:    err.Error(),
				Direction: direction,
			})
		}
	}

	return warnings
}

// gateFaultError marks a read-gate fault inside a larger operation (a relation
// peer check, a view's entry read), so the handler answers it through
// writeGateError instead of the operation's own error shape. That keeps the
// raw error off the wire, and keeps a peer fault from becoming a
// target_not_found warning that misreports a live peer as absent.
type gateFaultError struct{ err error }

func (e *gateFaultError) Error() string { return "read gate: " + e.err.Error() }
func (e *gateFaultError) Unwrap() error { return e.err }

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
func (h *writeHandler) applyRelationsModern(
	ctx context.Context, addr entity.Ref, desired map[string]v1.RelationsUpdate,
) ([]Warning, error) {
	entityID := addr.ID
	if len(desired) == 0 {
		return nil, nil
	}
	meta := h.schema().Meta
	em := h.manager
	var warnings []Warning

	for bodyKey, upd := range desired {
		canonical, incoming, ok := resolveDirection(meta, bodyKey)
		if !ok {
			// validateRelationsModern already screened this; defensive.
			return warnings, &structuralError{
				Code:   "unknown_relation_type",
				Path:   "/relations/" + v1.JSONPointerEscape(bodyKey),
				Detail: fmt.Sprintf("relation type %q is not defined in the metamodel", bodyKey),
			}
		}
		relDef := meta.Relations[canonical]
		direction := directionLabel(incoming)
		dataPath := "/relations/" + v1.JSONPointerEscape(bodyKey)

		ops, err := h.planEdges(ctx, entityID, addr.Face, canonical, incoming, upd, dataPath)
		if err != nil {
			return warnings, err
		}
		if err := h.requireReadablePeers(ctx, canonical, ops); err != nil {
			return warnings, err
		}
		if err := checkFullLinkageSize(&relDef, canonical, incoming, upd, dataPath); err != nil {
			return warnings, err
		}
		if create, ok := replaceCreate(&relDef, incoming, upd, ops); ok {
			// Re-pointing a single-valued relation: one atomic replace
			// instead of a delete and a create (TKT-65LVAK).
			finalProps, _, _ := mergeEdgeMeta(nil, create.ref)
			warnings = append(warnings, requiredMetaWarnings(canonical, &relDef, create.ref, finalProps,
				dataPath+upsertKey(upd), direction)...)
			if err := h.writeReplaceRelation(ctx, entityID, create, canonical, finalProps); err != nil {
				return warnings, err
			}
			continue
		}
		for _, op := range ops {
			k := edgeKeyOf(entityID, canonical, op.slot, incoming)
			if op.remove {
				err := em.DeleteRelation(ctx, k)
				if errors.Is(err, store.ErrNotFound) {
					continue // a concurrent request deleted it first
				}
				if err != nil {
					return warnings, &relationError{
						RelType: canonical, Target: op.slot.peer, Op: "delete",
						Reason: "delete_failed", Err: err,
					}
				}
				continue
			}
			finalProps, finalContent, contentSet := mergeEdgeMeta(op.existing, op.ref)
			warnings = append(warnings, requiredMetaWarnings(canonical, &relDef, op.ref, finalProps,
				dataPath+upsertKey(upd), direction)...)
			exists := op.existing != nil
			if exists && isEdgeNoOp(op.existing, finalProps, finalContent, contentSet, op.ref) {
				continue // value-based no-op suppression
			}
			// An existing edge is addressed by the tail it carries, never one
			// recomputed from the request: the store treats the tail as
			// identity, so a recomputed one would modify a different edge.
			if err := h.upsertEdge(ctx, edgeWrite{
				from: k.From, to: k.To, relType: canonical, ref: op.ref,
				exists: exists, existingTail: op.slot.tail, newTail: op.slot.tail,
				props: finalProps, content: finalContent,
			}); err != nil {
				return warnings, err
			}
		}
	}
	return warnings, nil
}

// requireReadablePeers refuses the upserts of ops whose peer has no face the
// principal may read. The manager checks only that a peer exists, so without
// this a hidden peer would be linked, or its existing edge updated, while an
// absent one is refused. Both get the dangling-peer error, before the manager
// runs, so its ACL cannot answer them differently either. A remove needs no
// check: the planner matches only edges whose endpoints the principal reads.
// One read gate covers every peer of ops.
func (h *writeHandler) requireReadablePeers(ctx context.Context, relType string, ops []edgeOp) error {
	var ids []string
	for _, op := range ops {
		if !op.remove {
			ids = append(ids, op.slot.peer)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	types, err := h.visible.readableTypes(ctx, ids)
	if err != nil {
		return &gateFaultError{err: err}
	}
	for _, op := range ops {
		if !op.remove && types[op.slot.peer] == "" {
			return danglingPeerError(relType, op.ref.ID)
		}
	}
	return nil
}

// edgeWrite is one desired edge for upsertEdge.
type edgeWrite struct {
	from, to, relType     string
	ref                   v1.ResourceIdentifier
	exists                bool // the edge existed when the reconciler read it
	existingTail, newTail entity.Face
	props                 map[string]any
	content               string
}

// upsertEdge writes one desired edge: an update when it existed at read time,
// a create otherwise. That read is not held still, so a concurrent request may
// create or delete the same edge in between. A write that finds the edge in
// the other state switches to the other write, up to upsertEdgeAttempts
// times. A caller still losing after that gets the last error, which
// writeRelationsApplyError answers as a 409.
func (h *writeHandler) upsertEdge(ctx context.Context, w edgeWrite) error {
	exists, tail := w.exists, w.existingTail
	var err error
	for range upsertEdgeAttempts {
		if exists {
			err = h.writeUpdateRelation(ctx, w.from, tail, w.to, w.relType, w.ref)
			if !errors.Is(err, entitymanager.ErrRelationNotFound) {
				return err
			}
		} else {
			err = h.writeCreateRelation(ctx, w.from, w.newTail, w.to, w.relType, w.ref, w.props, w.content)
			if !errors.Is(err, entitymanager.ErrRelationAlreadyExists) {
				return err
			}
		}
		exists, tail = !exists, w.newTail
	}
	return err
}

// upsertEdgeAttempts bounds how often upsertEdge switches between update and
// create while concurrent requests keep flipping the edge.
const upsertEdgeAttempts = 8

// writeCreateRelation creates a new relation via the EntityManager
// (preserving the ACL, audit, validation and automation paths).
//
// Error handling splits three ways:
//
//   - ACL denial (*acl.ForbiddenError): propagated so the handler maps it
//     to 403. The EntityManager authorizes BEFORE any peer-existence check
//     (BUG-K6FEVB), so a denied write is a ForbiddenError even when the
//     peer is missing — it never reaches the fallback below.
//   - Missing peer (source/target entity not found): returned as a hard
//     structuralError so the handler maps it to 422. This is a DELIBERATE
//     reversal of DEC-HWZHA's soft-condition treatment for THIS case: the
//     old ungated fallback wrote the edge directly to the store, skipping
//     the ACL and audit (that is the --read-only bypass BUG-K6FEVB). The
//     user needs feedback that the reference did not resolve, so we reject
//     rather than silently warn — and we NEVER write directly to the store.
//   - Type-allowlist mismatch (invalid relation): still a soft condition —
//     both endpoints exist and a hand-editor could produce this state — so
//     it keeps DEC-HWZHA's write-with-warning behavior via a direct store
//     write. This write is safe because the ACL already allowed it above.
//
// `from` and `to` are pre-resolved by the caller via edgeEndpoints —
// this function does not consult direction. `ref.ID` is the peer ID
// (which is `to` for outgoing edges and `from` for incoming).
func (h *writeHandler) writeCreateRelation(
	ctx context.Context, from string, tail entity.Face, to, relType string, ref v1.ResourceIdentifier,
	finalProps map[string]any, finalContent string,
) error {
	opts := entity.RelationOptions{
		Properties: finalProps,
		Content:    ref.Content,
	}
	k := entity.RelationKey{From: from, FromFace: tail, Type: relType, To: to}
	_, err := h.manager.CreateRelation(ctx, k, opts)
	if err == nil {
		return nil
	}
	if isMissingPeerCondition(err) {
		return danglingPeerError(relType, ref.ID)
	}
	if ce := cardinalityError(err, relType); ce != nil {
		return ce
	}
	if !isSoftCondition(err) {
		return &relationError{
			RelType: relType, Target: ref.ID, Op: "create",
			Reason: "create_failed", Err: err,
		}
	}
	// Soft condition (type-allowlist mismatch): write directly through
	// the store, skipping the workspace's pre-write validation. Safe
	// because the EntityManager already ran the ACL above.
	data := &store.RelationData{Properties: finalProps, Content: finalContent}
	sErr := h.store.Tx(ctx, func(view store.Store) error {
		if err := entitymanager.CheckRelationCapacity(ctx, view, h.schema().Meta, k); err != nil {
			return err
		}
		_, err := view.CreateRelation(ctx, k, data)
		return err
	})
	if ce := cardinalityError(sErr, relType); ce != nil {
		return ce
	}
	if sErr != nil {
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
func (h *writeHandler) writeUpdateRelation(
	ctx context.Context, from string, tail entity.Face, to, relType string, ref v1.ResourceIdentifier,
) error {
	opts := entity.RelationOptions{
		Properties: ref.Meta,
		MetaUnset:  ref.MetaUnset,
		Content:    ref.Content,
	}
	_, err := h.manager.UpdateRelation(ctx, entity.RelationKey{From: from, FromFace: tail, Type: relType, To: to}, opts)
	if err == nil {
		return nil
	}
	if isMissingPeerCondition(err) {
		// See writeCreateRelation: a dangling peer is a hard 422, never a
		// silent ungated store write (BUG-K6FEVB).
		return danglingPeerError(relType, ref.ID)
	}
	if !isSoftCondition(err) {
		return &relationError{
			RelType: relType, Target: ref.ID, Op: "update",
			Reason: "update_failed", Err: err,
		}
	}
	// Soft condition: rebuild the post-merge state and write directly.
	//
	// FAILS CLOSED. mergeEdgeMeta reads a nil `current` as "no prior state",
	// so swallowing a store error here would ERASE the edge's existing
	// properties instead of merging into them — a silent partial write on a
	// transient fault.
	//
	// The read, merge and write share one Tx, like the manager's
	// UpdateRelation, so a concurrent update cannot land in between and be
	// overwritten by the merge of the row read before it.
	txErr := h.store.Tx(ctx, func(view store.Store) error {
		current, readErr := view.GetRelation(ctx, entity.RelationKey{From: from, FromFace: tail, Type: relType, To: to})
		if readErr != nil && !errors.Is(readErr, store.ErrNotFound) {
			return readErr
		}
		finalProps, finalContent, _ := mergeEdgeMeta(current, ref)
		data := store.RelationData{Properties: finalProps, Content: finalContent}
		_, sErr := view.UpdateRelation(ctx, entity.RelationKey{From: from, FromFace: tail, Type: relType, To: to}, data)
		return sErr
	})
	if txErr != nil {
		return &relationError{
			RelType: relType, Target: ref.ID, Op: "update",
			Reason: "update_failed", Err: txErr,
		}
	}
	return nil
}

// incomingEdgeTail reports the tail of the stored edge from→to, for the
// INCOMING path of the single-relation PATCH and DELETE routes. There the
// edge's source is the peer, so its tail is not this request's to choose:
// recomputing it from the request would address a DIFFERENT edge and report
// success (RR-5MLZCR). An edge the path entity owns takes its tail from
// [writeHandler.ownedTail] instead.
//
// The peer may hold one edge per face, so the target may name the face
// (`POL-1@draft`). Only edges the principal can read are candidates
// ([visibleReader.readableRelations]). A bare target names the one such
// edge, and is face_required when there are several. No match returns the
// zero face, which leaves the manager's own not-found answer to stand; a
// named face the principal cannot read is [errEdgeNotFound], the same answer
// as an absent edge. A read fault is returned: it is not evidence of a
// default-tail edge, and the affordance gate reads the source at this tail.
func (h *writeHandler) incomingEdgeTail(
	ctx context.Context, peer entity.Address, relType, to string,
) (entity.Face, error) {
	from := peer.ID()
	var edges []*entity.Relation
	for rel, err := range h.store.ListRelations(ctx, store.RelationQuery{From: from, Type: relType, To: to}) {
		if err != nil {
			return "", err
		}
		if rel.From == from && rel.Type == relType && rel.To == to {
			edges = append(edges, rel)
		}
	}
	edges, err := h.visible.readableRelations(ctx, edges)
	if err != nil {
		return "", &gateFaultError{err: err}
	}
	if named, ok := peer.Named(); ok {
		for _, rel := range edges {
			if rel.FromFace == named.Face {
				return named.Face, nil
			}
		}
		return "", errEdgeNotFound
	}
	switch len(edges) {
	case 0:
		return entity.ImplicitFace, nil
	case 1:
		return edges[0].FromFace, nil
	}
	return "", &structuralError{
		Code: "face_required", Path: "/relations/" + v1.JSONPointerEscape(relType) + "/" + from,
		Detail: fmt.Sprintf("%s links here from more than one face; name the face, as %s",
			from, entity.FormatStateRef(from, edges[0].FromFace)),
	}
}

// errEdgeNotFound is [writeHandler.incomingEdgeTail]'s answer for an edge
// that does not exist or that the principal cannot read.
var errEdgeNotFound = errors.New("relation not found")

// isMissingPeerCondition reports whether the EntityManager error is a
// dangling-peer error (source or target entity does not exist). This is
// treated as a HARD 422, not a soft warning (BUG-K6FEVB) — see
// danglingPeerError and writeCreateRelation for the rationale.
func isMissingPeerCondition(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "target entity not found") ||
		strings.Contains(msg, "source entity not found")
}

// isSoftCondition returns true when the error from EntityManager
// indicates a DEC-HWZHA "soft" condition that should be treated as a
// warning rather than blocking the write. The current workspace
// implementation surfaces these as plain fmt.Errorf strings; we match
// on substrings, which is fragile but acceptable for the current
// implementation surface.
//
// A missing peer is NOT a soft condition here (see isMissingPeerCondition);
// only the type-allowlist mismatch remains soft, because both endpoints
// exist and a hand-editor could produce that state.
func isSoftCondition(err error) bool {
	if err == nil {
		return false
	}
	// metamodel.ValidateRelation rejects type-allowlist failures.
	return strings.Contains(err.Error(), "invalid relation:")
}

// danglingPeerError builds the hard 422 returned when a relation write
// references a peer entity that does not exist. A structuralError maps to
// HTTP 422 via writeRelationsApplyError, telling the user the reference
// did not resolve (and so the edge was NOT stored). This deliberately
// reverses DEC-HWZHA's soft-warn treatment for the missing-peer case: the
// old behavior wrote the edge through an ungated direct store call that
// bypassed the ACL and audit (BUG-K6FEVB, the --read-only bypass).
func danglingPeerError(relType, peerID string) *structuralError {
	return &structuralError{
		Code:   "target_not_found",
		Path:   "/relations/" + v1.JSONPointerEscape(relType) + "/data",
		Detail: fmt.Sprintf("relation %q references entity %q, which does not exist", relType, peerID),
	}
}

// mergeEdgeMeta computes the post-merge (properties, content) tuple
// for an edge, given the existing relation (or nil for new edges) and
// the desired ref. contentSet reports whether ref.Content was non-nil
// (so the caller can distinguish "leave alone" from "set to value").
func mergeEdgeMeta(existing *entity.Relation, ref v1.ResourceIdentifier) (
	props map[string]any, content string, contentSet bool,
) {
	props = make(map[string]any)
	if existing != nil {
		maps.Copy(props, existing.Properties)
		content = existing.Content
	}
	maps.Copy(props, ref.Meta)
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
	existing *entity.Relation, finalProps map[string]any,
	finalContent string, contentSet bool, ref v1.ResourceIdentifier,
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
func mapsEqual(a, b map[string]any) bool {
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
	relType string, relDef *metamodel.RelationDef, ref v1.ResourceIdentifier,
	finalProps map[string]any, dataPath, direction string,
) []Warning {
	var ws []Warning
	for k, propDef := range relDef.Properties {
		if !propDef.Required {
			continue
		}
		if _, ok := finalProps[k]; !ok {
			ws = append(ws, Warning{
				Code:      "required_meta_unset",
				Path:      fmt.Sprintf("%s[id=%s]/meta/%s", dataPath, v1.JSONPointerEscape(ref.ID), v1.JSONPointerEscape(k)),
				Detail:    fmt.Sprintf("relation type %q requires meta property %q", relType, k),
				Direction: direction,
			})
		}
	}
	return ws
}

// validateManagedOrderMeta enforces that values written into the managed
// order properties (_order_out / _order_in) are finite numeric. Non-finite
// or non-numeric values are wire-format violations (400) — the engine's
// midpoint and sort logic depends on finite float64 values.
func validateManagedOrderMeta(
	relType string, ref v1.ResourceIdentifier, edgePath string, relDef *metamodel.RelationDef,
) error {
	check := func(prop string) error {
		v, present := ref.Meta[prop]
		if !present {
			return nil
		}
		if _, ok := entitymanager.FiniteOrder(v); ok {
			return nil
		}
		return &v1.WireError{
			Code:   "order_value_invalid",
			Path:   edgePath + "/meta/" + v1.JSONPointerEscape(prop),
			Detail: fmt.Sprintf("relation type %q managed order property %q must be a finite number", relType, prop),
		}
	}
	if relDef.OutgoingOrderProperty() != "" {
		if err := check(metamodel.OrderPropertyOut); err != nil {
			return err
		}
	}
	if relDef.IncomingOrderProperty() != "" {
		if err := check(metamodel.OrderPropertyIn); err != nil {
			return err
		}
	}
	return nil
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

// replaceCreate reports whether a relation wrapper re-points a single-valued
// relation (TKT-65LVAK): a full linkage (`data`) on an outgoing relation with
// `max_outgoing: 1` whose plan is one create plus removals. It returns the
// create. Any other plan (a no-op, a meta update of the kept edge, a delta)
// goes through the edge-by-edge loop, which cannot exceed the bound there.
func replaceCreate(
	relDef *metamodel.RelationDef, incoming bool, upd v1.RelationsUpdate, ops []edgeOp,
) (edgeOp, bool) {
	if incoming || upd.Delta || !upd.DataPresent || relDef.MaxOutgoing == nil || *relDef.MaxOutgoing != 1 {
		return edgeOp{}, false
	}
	var create edgeOp
	creates := 0
	for _, op := range ops {
		switch {
		case op.remove:
		case op.existing == nil:
			create = op
			creates++
		default:
			return edgeOp{}, false
		}
	}
	return create, creates == 1
}

// checkFullLinkageSize refuses a full linkage naming more targets than the
// relation's max_outgoing allows, before any edge is written.
func checkFullLinkageSize(
	relDef *metamodel.RelationDef, relType string, incoming bool, upd v1.RelationsUpdate, path string,
) error {
	if incoming || upd.Delta || !upd.DataPresent || relDef.MaxOutgoing == nil {
		return nil
	}
	if n := len(upd.Data); n > *relDef.MaxOutgoing {
		return &structuralError{
			Code: "cardinality_exceeded",
			Path: path + "/data",
			Detail: fmt.Sprintf("relation %q allows at most %d target(s) (max_outgoing); the request names %d",
				relType, *relDef.MaxOutgoing, n),
		}
	}
	return nil
}

// cardinalityError maps a refused create (entitymanager.ErrCardinalityExceeded)
// to a 422 naming the relation and its bound; nil for any other error.
func cardinalityError(err error, relType string) *structuralError {
	var ce *entitymanager.CardinalityError
	if !errors.As(err, &ce) {
		return nil
	}
	return &structuralError{
		Code:   "cardinality_exceeded",
		Path:   "/relations/" + v1.JSONPointerEscape(relType),
		Detail: ce.Error(),
	}
}

// writeReplaceRelation applies a replaceCreate plan through
// [entitymanager.Manager.ReplaceOutgoing], mapping its errors the way
// writeCreateRelation maps a create's.
func (h *writeHandler) writeReplaceRelation(
	ctx context.Context, entityID string, create edgeOp, relType string, props map[string]any,
) error {
	k := entity.RelationKey{From: entityID, FromFace: create.slot.tail, Type: relType, To: create.slot.peer}
	_, err := h.manager.ReplaceOutgoing(ctx, k, entity.RelationOptions{Properties: props, Content: create.ref.Content})
	if err == nil {
		return nil
	}
	if isMissingPeerCondition(err) {
		return danglingPeerError(relType, create.ref.ID)
	}
	if ce := cardinalityError(err, relType); ce != nil {
		return ce
	}
	return &relationError{RelType: relType, Target: create.ref.ID, Op: "replace", Reason: "replace_failed", Err: err}
}
