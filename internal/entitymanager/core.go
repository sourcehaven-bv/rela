package entitymanager

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
)

// createCoreOpts configures core entity creation — the bare write
// path used by both [Manager.CreateEntity] (which wraps it with
// automation + cascade) and [cascadeHost.CreateEntity] (which calls
// it directly to avoid recursive cascade).
type createCoreOpts struct {
	ID              string                 // Custom ID (empty = auto-generate)
	IDPrefix        string                 // Prefix for auto-generated ID
	TemplateVariant string                 // Template variant name (empty = default)
	Properties      map[string]interface{} // Properties to set (overrides template defaults)
	Content         string                 // Body content (overrides template content when non-empty)
	// SkipIDGeneration tells [buildCandidateEntity] to skip real ID
	// allocation when ID is empty and the type uses auto-IDs.
	// [Manager.ValidateCreate] sets this so a per-keystroke dry-run does
	// not scan the entire store to pick a never-used ID. The resulting
	// entity gets a stable placeholder ID; validation that depends on
	// the actual ID (ID-prefix check) is not relevant in that path.
	SkipIDGeneration bool
}

// resolveCandidateID returns the ID to use for the candidate entity
// being built by [buildCandidateEntity]. Three branches:
//   - User-supplied ID → validated (manual-ID types only).
//   - Empty ID + SkipIDGeneration → synthesized placeholder
//     (dry-run / validation path; no store scan).
//   - Empty ID + auto-ID → generated via the full-store scan.
//
// Extracted from [buildCandidateEntity] to keep that function's
// top-level flow flat (avoids the nestif lint warning).
func resolveCandidateID(
	ctx context.Context, deps Deps, entityType string, entityDef *metamodel.EntityDef, opts createCoreOpts,
) (string, error) {
	if opts.ID != "" {
		if !entityDef.IsManualID() {
			return "", customIDNotAllowedError(entityType, entityDef, opts.ID)
		}
		if err := entity.ValidateID(opts.ID); err != nil {
			return "", err
		}
		return opts.ID, nil
	}
	if opts.SkipIDGeneration {
		// Dry-run / validation path: skip the full-store scan
		// generateID would do. The placeholder must pass metamodel
		// ID-prefix validation; [candidatePlaceholderID] synthesizes
		// one from the type's first prefix. Never persisted — only
		// [Manager.ValidateCreate] sets SkipIDGeneration.
		return candidatePlaceholderID(entityDef, opts.IDPrefix), nil
	}
	return generateID(ctx, deps, entityType, opts.IDPrefix)
}

// candidatePlaceholderID returns the synthetic ID to use for a dry-run
// candidate entity when no real ID is supplied. It pairs the requested
// prefix (or the type's first declared prefix) with a fixed suffix so
// metamodel ID-prefix validation passes. The result is never persisted
// — only [Manager.ValidateCreate] (which never writes) uses this path.
func candidatePlaceholderID(def *metamodel.EntityDef, requestedPrefix string) string {
	const candidateSuffix = "DRYRUN"
	if requestedPrefix != "" {
		return requestedPrefix + candidateSuffix
	}
	if prefixes := def.GetIDPrefixes(); len(prefixes) > 0 {
		return prefixes[0] + candidateSuffix
	}
	return candidateSuffix
}

// createCore is the shared bare-write entity-creation path: resolve
// ID, apply template defaults, apply caller properties, partition
// validation errors per DEC-HWZHA (hard errors abort; soft conditions
// proceed and are returned as warnings), persist. **No automation.**
//
// Free function over [Deps] (not a method on Manager) so cascadeHost
// can call it directly without constructing a half-initialized
// Manager view.
func createCore(
	ctx context.Context, deps Deps, entityType string, opts createCoreOpts,
) (*entity.Entity, []entity.Warning, error) {
	e, warnings, err := buildCandidateEntity(ctx, deps, entityType, opts)
	if err != nil {
		return nil, nil, err
	}

	if err := upsertEntity(ctx, deps.Store, e); err != nil {
		return nil, nil, fmt.Errorf("write entity: %w", err)
	}

	return e, warnings, nil
}

// buildCandidateEntity resolves the ID, applies template + status
// defaults, merges caller properties, and partitions validation errors
// per DEC-HWZHA (hard errors abort; soft conditions return as warnings)
// — everything [createCore] does except the final persist. Shared by
// createCore (which then writes) and [Manager.ValidateCreate] (which
// returns the would-be entity + warnings without writing), so dry-run
// validation cannot drift from the real create path.
func buildCandidateEntity(
	ctx context.Context, deps Deps, entityType string, opts createCoreOpts,
) (*entity.Entity, []entity.Warning, error) {
	entityDef, ok := deps.Meta.GetEntityDef(entityType)
	if !ok {
		return nil, nil, fmt.Errorf("unknown entity type: %s", entityType)
	}

	entityID, err := resolveCandidateID(ctx, deps, entityType, entityDef, opts)
	if err != nil {
		return nil, nil, err
	}

	e := entity.New(entityID, entityType)

	tmpl, err := deps.Templater.EntityTemplate(ctx, entityType, opts.TemplateVariant)
	if err != nil {
		return nil, nil, fmt.Errorf("load template: %w", err)
	}
	if opts.TemplateVariant != "" && tmpl == nil {
		return nil, nil, fmt.Errorf("template variant %q not found for entity type %s", opts.TemplateVariant, entityType)
	}
	if tmpl != nil {
		e.Properties, e.Content = templating.ApplyEntity(e.Properties, e.Content, tmpl)
	}

	for k, v := range opts.Properties {
		e.Properties[k] = v
	}

	if opts.Content != "" {
		e.Content = opts.Content
	}

	if e.GetString("status") == "" {
		e.SetString("status", entityDef.GetDefaultStatus(deps.Meta))
	}

	// DEC-HWZHA: hard structural errors abort; soft conditions
	// (required-missing, type mismatch, invalid enum, malformed value)
	// ride along on the result as warnings.
	var warnings []entity.Warning
	if errs := deps.Meta.ValidateEntity(e.ID, e.Type, e.Properties); len(errs) > 0 {
		hard, soft := partitionValidationErrors(errs)
		if len(hard) > 0 {
			return nil, nil, newValidationError(hard)
		}
		warnings = soft
	}

	return e, warnings, nil
}

// generateID generates the next ID for the given entity type. If
// prefix is non-empty it overrides the metamodel-default prefix.
func generateID(ctx context.Context, deps Deps, entityType, prefix string) (string, error) {
	entityDef, ok := deps.Meta.GetEntityDef(entityType)
	if !ok {
		return "", fmt.Errorf("unknown entity type: %s", entityType)
	}
	if entityDef.IsManualID() {
		return "", fmt.Errorf("entity type %s uses manual IDs", entityType)
	}
	if prefix == "" {
		prefixes := entityDef.GetIDPrefixes()
		if len(prefixes) == 0 {
			return "", fmt.Errorf("no ID prefixes defined for type %s", entityType)
		}
		prefix = prefixes[0]
	}

	existingIDs := collectAllIDs(ctx, deps.Store)
	if entityDef.IsShortID() {
		return entity.GenerateShortID(existingIDs, prefix, len(existingIDs), entityDef.GetIDCaps()), nil
	}
	return entity.GenerateNextID(existingIDs, prefix), nil
}

// collectAllIDs returns every entity ID currently in the store.
// Errors from the iterator are swallowed — a partial result is
// preferable to failing ID generation outright.
func collectAllIDs(ctx context.Context, st store.Store) []string {
	ids := make([]string, 0)
	for e, err := range st.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			return ids
		}
		ids = append(ids, e.ID)
	}
	return ids
}

// collectIncidentRelations gathers a store's relations in the given
// direction for the given entity. Errors from the iterator are
// swallowed — partial results are preferable to a failing cascade.
func collectIncidentRelations(
	ctx context.Context, st store.Store, id string, dir store.Direction,
) []*entity.Relation {
	out := make([]*entity.Relation, 0)
	for r, err := range st.ListRelations(ctx, store.RelationQuery{
		EntityID:  id,
		Direction: dir,
	}) {
		if err != nil {
			continue
		}
		out = append(out, r)
	}
	return out
}

// findExistingRelationTarget locates an existing target entity of the
// given type that is the target of a relation from sourceID with the
// given relationType. Returns nil if none exists.
func findExistingRelationTarget(
	ctx context.Context, st store.Store, sourceID, relationType, targetType string,
) *entity.Entity {
	for rel, err := range st.ListRelations(ctx, store.RelationQuery{
		EntityID:  sourceID,
		Direction: store.DirectionOutgoing,
		Type:      relationType,
	}) {
		if err != nil {
			continue
		}
		target, getErr := st.GetEntity(ctx, rel.To)
		if getErr != nil {
			continue
		}
		if target.Type == targetType {
			return target
		}
	}
	return nil
}

// upsertEntity persists an entity to the store. Tries CreateEntity
// first; if and only if that fails with [store.ErrConflict], falls
// back to UpdateEntity. Any other error from CreateEntity is
// returned as-is — masking a permission or I/O failure as a missing
// entity would cause confusing downstream errors.
func upsertEntity(ctx context.Context, st store.Store, e *entity.Entity) error {
	if e == nil {
		return nil
	}
	err := st.CreateEntity(ctx, e)
	if err == nil {
		return nil
	}
	if !errors.Is(err, store.ErrConflict) {
		return err
	}
	return st.UpdateEntity(ctx, e)
}

// upsertRelation persists a relation to the store with the same
// "Create-then-Update on ErrConflict" discipline as upsertEntity.
func upsertRelation(ctx context.Context, st store.Store, r *entity.Relation) error {
	if r == nil {
		return nil
	}
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
		return fmt.Errorf("update relation: %w", uErr)
	}
	return nil
}
