package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"

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
	ID              string         // Custom ID (empty = auto-generate)
	IDPrefix        string         // Prefix for auto-generated ID
	TemplateVariant string         // Template variant name (empty = default)
	Properties      map[string]any // Properties to set (overrides template defaults)
	Content         string         // Body content (overrides template content when non-empty)
	// Face is the content state to write. Carried here rather than read off
	// the caller's entity so the face this authorizes and the face it writes
	// are provably the same value (BUG-HC6I2T: they were populated from
	// different places, and only one was ever set).
	Face entity.Face
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

	// Enforce state-machine entry BEFORE the durable write (TKT-E4LW2,
	// RR-HETEE): the candidate carries the resolved post-template property
	// values, so the entry-value check needs no persisted row. Running it
	// here — not after Store.CreateEntity — means an illegal entry is
	// rejected without ever emitting a store event (no orphaned index entry,
	// no SSE broadcast, no un-audited row).
	if err := deps.Transitions.EnforceCreate(ctx, e); err != nil {
		return nil, nil, err
	}

	// Enforce `unique: true` natural-key constraints before the durable
	// write. excludeSelfID is empty: on create there is no prior version
	// of this entity to exclude.
	if err := checkUniqueProperties(ctx, deps, e, ""); err != nil {
		return nil, nil, err
	}

	// A create must never fall through to an update — that would
	// overwrite a colliding entity (a racing create, or a stale-scan
	// duplicate ID). Write with a direct CreateEntity and surface a
	// conflict as ErrEntityAlreadyExists. Every write path is now
	// create-XOR-update by resolved intent; there is no create-then-
	// update-on-conflict fallback anywhere (BUG-ZWTDH9).
	if err := deps.Store.CreateEntity(ctx, e); err != nil {
		// A derived unique-property index (pgstore, TKT-3Q0GP1) rejects a
		// duplicate PROPERTY value; surface it as the same 422 the pre-write
		// scan does. Check this BEFORE the ErrConflict branch: UniquePropertyError
		// satisfies errors.Is(_, ErrConflict) too, and the ID-collision message
		// would be wrong for a property duplicate.
		if ok, mapped := mapUniquePropertyConflict(err); ok {
			return nil, nil, mapped
		}
		if errors.Is(err, store.ErrConflict) {
			return nil, nil, fmt.Errorf("%w: %s", ErrEntityAlreadyExists, e.ID)
		}
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
	e.Face = opts.Face

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

	maps.Copy(e.Properties, opts.Properties)
	if err := rejectComputedPresent(deps, entityType, e.Properties); err != nil {
		return nil, nil, err
	}

	if opts.Content != "" {
		e.Content = opts.Content
	}

	if e.GetString("status") == "" {
		e.SetString("status", entityDef.GetDefaultStatus(deps.Meta))
	}
	if err := deps.Computed.Evaluate(ctx, e); err != nil {
		return nil, nil, err
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

	existingIDs, err := collectAllIDs(ctx, deps.Store)
	if err != nil {
		return "", fmt.Errorf("collect existing IDs: %w", err)
	}
	if entityDef.IsShortID() {
		return entity.GenerateShortID(existingIDs, prefix, len(existingIDs), entityDef.GetIDCaps()), nil
	}
	return entity.GenerateNextID(existingIDs, prefix), nil
}

// collectAllIDs returns every entity ID currently in the store.
// A partial scan must not feed ID generation: a truncated list can hide
// a high-numbered existing ID, so the generator would mint one that
// already exists. createCore then rejects that collision with
// ErrEntityAlreadyExists (a create never overwrites), so a truncated
// scan would surface as a spurious conflict rather than data loss — but
// fail loudly here regardless, so the generator is never fed bad data.
// AllStates, then deduped by bare id. Without it the scan sees only
// zero-coordinate rows, which a type declaring faces has none of — so the
// generator saw an EMPTY id set for such a type and minted the same id for
// every entity of it (BUG-HC6I2T). Counting a family once is what the query
// used to get from the zero coordinate; with no face privileged, dedup is
// what provides it.
func collectAllIDs(ctx context.Context, st store.Store) ([]string, error) {
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	for e, err := range st.ListEntities(ctx, store.EntityQuery{AllStates: true}) {
		if err != nil {
			return nil, err
		}
		if _, dup := seen[e.ID]; dup {
			continue
		}
		seen[e.ID] = struct{}{}
		ids = append(ids, e.ID)
	}
	return ids, nil
}

// collectIncidentRelations gathers a store's relations in the given
// direction for the given entity. The result gates the delete-safety
// check (refuse a non-cascade delete that would orphan relations), so a
// partial scan must not silently under-count — propagate the error and
// let the caller refuse the delete rather than proceed on partial data.
func collectIncidentRelations(
	ctx context.Context, st store.Store, id string, dir store.Direction,
) ([]*entity.Relation, error) {
	return collectRelations(ctx, st, store.RelationQuery{
		EntityID:  id,
		Direction: dir,
	})
}

// collectRelations drains a relation query into a slice, failing on the
// first store error rather than returning a partial set.
func collectRelations(
	ctx context.Context, st store.Store, q store.RelationQuery,
) ([]*entity.Relation, error) {
	out := make([]*entity.Relation, 0)
	for r, err := range st.ListRelations(ctx, q) {
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
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

// requireCreateFaceFor enforces that a create names exactly the faces its type
// declares: one of them for a faced type, none for a faceless one.
//
// The rule is symmetric on purpose. A faced type has no zero-coordinate row to
// default to, and a faceless type has no name for the single state it does
// store, so in both directions the only safe answer is to refuse rather than
// pick. Reads still resolve a bare address through the world chain; a WRITE
// never does, because a chain answers with a fallback and a write must name
// the row it changes (BUG-HC6I2T).
func (d Deps) requireCreateFaceFor(entityType string, face entity.Face) error {
	def, ok := d.Meta.GetEntityDef(entityType)
	if !ok {
		return fmt.Errorf("unknown entity type: %s", entityType)
	}
	if len(def.Faces) == 0 {
		if !face.IsDefault() {
			return fmt.Errorf("%w: %s declares no faces, so %q names nothing",
				ErrFaceNotDeclared, entityType, face)
		}
		return nil
	}
	if face.IsDefault() {
		return fmt.Errorf("%w: %s declares %s", ErrFaceRequired,
			entityType, strings.Join(sortedFaceNames(def), ", "))
	}
	if _, declared := def.Faces[face.String()]; !declared {
		return fmt.Errorf("%w: %s declares %s, not %q", ErrFaceNotDeclared,
			entityType, strings.Join(sortedFaceNames(def), ", "), face)
	}
	return nil
}

// sortedFaceNames lists a type's declared faces in a stable order, so an
// error message names them the same way twice.
func sortedFaceNames(def *metamodel.EntityDef) []string {
	names := make([]string, 0, len(def.Faces))
	for name := range def.Faces {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// anyFaceOf returns any stored row of an entity, for the checks that ask a
// question every face of a family answers the same way — its TYPE.
//
// Relation endpoints are the motivating case. A relation attaches to the
// ENTITY, not to one of its content states (`scope: identity`), so validating
// `from`/`to` needs the family's type and nothing face-specific. Reading that
// with Store.GetEntity asks the ZERO coordinate, where a type declaring faces
// stores no row at all — so every relation touching a faced entity reported
// the entity as missing (BUG-HC6I2T).
//
// Prefers the addressed row when `id` carries a face, so a caller who named
// one is answered about it; otherwise takes the first row the family has.
// Which row that is does not matter: the callers use only the type, and every
// state of a family shares it (the store refuses a divergent one).
//
// FAILS CLOSED. Only a genuine [store.ErrNotFound] falls through to the next
// lookup; any other error is returned as-is. Swallowing a transient backend
// error here would report the entity as missing, and every caller's not-found
// branch skips the ACL check by design (existence is itself a secret), so a
// store hiccup would turn an ACL-gated operation into an ungated one — the
// defect TestRename_FailsClosedOnNonNotFoundFetchError pins.
//
// Nil: never returned with a nil error.
func anyFaceOf(ctx context.Context, st store.Store, id string) (*entity.Entity, error) {
	e, err := st.GetEntity(ctx, id)
	if err == nil {
		return e, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	base, face, perr := entity.ParseStateRef(id)
	if perr == nil && !face.IsDefault() {
		faced, serr := st.GetEntityState(ctx, base, face)
		if serr == nil {
			return faced, nil
		}
		if !errors.Is(serr, store.ErrNotFound) {
			return nil, serr
		}
	} else {
		base = id
	}
	// IDs-scoped, never a full scan: this runs on the relation write path,
	// once per endpoint, and an unbounded scan there is the per-row lookup
	// the collection-read rules exist to prevent.
	q := store.EntityQuery{IDs: []string{base}, AllStates: true}
	for e, err := range st.ListEntities(ctx, q) {
		if err != nil {
			return nil, err
		}
		if e.ID == base {
			return e, nil
		}
	}
	return nil, store.ErrNotFound
}
