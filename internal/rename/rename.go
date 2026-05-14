// Package rename implements entity ID renames against a [store.Store]:
// it updates the entity, rewrites every incident relation (in either
// direction) to use the new ID, and removes the old entity and old
// relation files.
//
// Rename is intentionally a pure-store operation: no automation, no
// cascade, no metamodel re-validation. Callers (entitymanager.Manager
// and the legacy workspace shim during transition) embed it directly.
//
// **Best-effort semantics.** If a relation rewrite fails mid-way, the
// partial state stays on disk. The caller can re-run the same rename
// command to converge — every step is idempotent except for the
// terminal old-entity delete.
package rename

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Options configures the rename operation.
type Options struct {
	// DryRun returns a descriptive [Result] without persisting any
	// changes.
	DryRun bool

	// EntityType, when non-empty, is checked against the loaded
	// entity's type before any writes — an extra safety net for
	// callers that already know what type they expect. Empty means
	// "trust the store."
	EntityType string
}

// RelationRef identifies a relation by its three-part key.
type RelationRef struct {
	From string `json:"from"`
	Type string `json:"type"`
	To   string `json:"to"`
}

// Result describes what a [Rename] did (or, in DryRun mode, would do).
type Result struct {
	OldID            string
	NewID            string
	EntityType       string
	RelationsUpdated []RelationRef
}

// Sentinels for distinguishing common failure modes via [errors.Is].
var (
	ErrEntityNotFound      = errors.New("entity not found")
	ErrEntityAlreadyExists = errors.New("entity already exists")
	ErrEntityTypeMismatch  = errors.New("entity type mismatch")
)

// Rename renames an entity from oldID to newID, rewriting every
// incident relation. See package doc for semantics.
func Rename(ctx context.Context, st store.Store, oldID, newID string, opts Options) (*Result, error) {
	if vErr := validatePreconditions(ctx, st, oldID, newID); vErr != nil {
		return nil, vErr
	}

	oldEntity, err := st.GetEntity(ctx, oldID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, oldID)
	}
	if opts.EntityType != "" && oldEntity.Type != opts.EntityType {
		return nil, fmt.Errorf("%w: %s has type %s, not %s",
			ErrEntityTypeMismatch, oldID, oldEntity.Type, opts.EntityType)
	}

	incoming := collectIncident(ctx, st, oldID, store.DirectionIncoming)
	outgoing := collectIncident(ctx, st, oldID, store.DirectionOutgoing)

	result := buildResult(oldEntity.Type, oldID, newID, incoming, outgoing)
	if opts.DryRun {
		return result, nil
	}

	newEntity := &entity.Entity{
		ID:         newID,
		Type:       oldEntity.Type,
		Properties: oldEntity.Properties,
		Content:    oldEntity.Content,
	}
	if err := upsertEntity(ctx, st, newEntity); err != nil {
		return nil, fmt.Errorf("write new entity %s: %w", newID, err)
	}

	if err := writeRenamedRelations(ctx, st, oldID, newID, incoming, outgoing); err != nil {
		return nil, err
	}

	// Delete old relation files first, then the entity. Self-
	// referential relations are deleted exactly once via the outgoing
	// loop; the incoming loop skips them.
	for _, rel := range outgoing {
		if err := st.DeleteRelation(ctx, oldID, rel.Type, rel.To); err != nil {
			return nil, fmt.Errorf("delete old outgoing relation %s--%s-->%s: %w",
				oldID, rel.Type, rel.To, err)
		}
	}
	for _, rel := range incoming {
		if rel.From == oldID {
			continue
		}
		if err := st.DeleteRelation(ctx, rel.From, rel.Type, oldID); err != nil {
			return nil, fmt.Errorf("delete old incoming relation %s--%s-->%s: %w",
				rel.From, rel.Type, oldID, err)
		}
	}
	if _, err := st.DeleteEntity(ctx, oldID, false); err != nil {
		return nil, fmt.Errorf("delete old entity %s: %w", oldID, err)
	}

	return result, nil
}

// validatePreconditions checks: old entity exists, new ID is
// available, new ID is well-formed.
func validatePreconditions(ctx context.Context, st store.Store, oldID, newID string) error {
	if _, err := st.GetEntity(ctx, oldID); err != nil {
		return fmt.Errorf("%w: %s", ErrEntityNotFound, oldID)
	}
	if _, err := st.GetEntity(ctx, newID); err == nil {
		return fmt.Errorf("%w: %s", ErrEntityAlreadyExists, newID)
	}
	if err := entity.ValidateID(newID); err != nil {
		return fmt.Errorf("invalid new ID: %w", err)
	}
	return nil
}

// collectIncident gathers a store's relations for the given entity in
// the given direction. Errors from the iterator are swallowed —
// partial results are preferable to a hard failure.
func collectIncident(ctx context.Context, st store.Store, id string, dir store.Direction) []*entity.Relation {
	out := make([]*entity.Relation, 0)
	for r, err := range st.ListRelations(ctx, store.RelationQuery{EntityID: id, Direction: dir}) {
		if err != nil {
			continue
		}
		out = append(out, r)
	}
	return out
}

// buildResult assembles the descriptive [Result] from incident
// relations. Self-referential edges contribute two entries (one per
// direction) to match the pre-extraction workspace behavior pinned by
// TestRename_SelfReferential.
func buildResult(entityType, oldID, newID string, incoming, outgoing []*entity.Relation) *Result {
	r := &Result{
		OldID:            oldID,
		NewID:            newID,
		EntityType:       entityType,
		RelationsUpdated: make([]RelationRef, 0, len(incoming)+len(outgoing)),
	}
	for _, rel := range outgoing {
		r.RelationsUpdated = append(r.RelationsUpdated, RelationRef{From: newID, Type: rel.Type, To: rel.To})
	}
	for _, rel := range incoming {
		r.RelationsUpdated = append(r.RelationsUpdated, RelationRef{From: rel.From, Type: rel.Type, To: newID})
	}
	return r
}

// writeRenamedRelations rewrites every incident relation with the new
// ID substituted on the appropriate side. Self-referential relations
// are rewritten exactly once via the outgoing loop.
func writeRenamedRelations(
	ctx context.Context, st store.Store, oldID, newID string,
	incoming, outgoing []*entity.Relation,
) error {
	for _, rel := range outgoing {
		newTo := rel.To
		if rel.To == oldID {
			newTo = newID
		}
		newRel := &entity.Relation{
			From: newID, Type: rel.Type, To: newTo,
			Properties: rel.Properties, Content: rel.Content,
		}
		if err := upsertRelation(ctx, st, newRel); err != nil {
			return fmt.Errorf("write relation %s--%s-->%s: %w", newID, rel.Type, newTo, err)
		}
	}
	for _, rel := range incoming {
		if rel.From == oldID {
			continue
		}
		newRel := &entity.Relation{
			From: rel.From, Type: rel.Type, To: newID,
			Properties: rel.Properties, Content: rel.Content,
		}
		if err := upsertRelation(ctx, st, newRel); err != nil {
			return fmt.Errorf("write relation %s--%s-->%s: %w", rel.From, rel.Type, newID, err)
		}
	}
	return nil
}

// upsertEntity persists an entity, falling back to Update only on
// [store.ErrConflict] so non-conflict failures surface as-is.
func upsertEntity(ctx context.Context, st store.Store, e *entity.Entity) error {
	err := st.CreateEntity(ctx, e)
	if err == nil {
		return nil
	}
	if !errors.Is(err, store.ErrConflict) {
		return err
	}
	return st.UpdateEntity(ctx, e)
}

// upsertRelation persists a relation, falling back to Update only on
// [store.ErrConflict].
func upsertRelation(ctx context.Context, st store.Store, r *entity.Relation) error {
	_, err := st.CreateRelation(ctx, r.From, r.Type, r.To, &store.RelationData{
		Properties: r.Properties,
		Content:    r.Content,
	})
	if err == nil {
		return nil
	}
	if !errors.Is(err, store.ErrConflict) {
		return err
	}
	if _, uErr := st.UpdateRelation(ctx, r.From, r.Type, r.To, store.RelationData{
		Properties: r.Properties,
		Content:    r.Content,
	}); uErr != nil {
		return uErr
	}
	return nil
}
