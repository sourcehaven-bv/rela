package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
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
	// mintSpread widens a sequential ID's pick to a random number in
	// [max+1, max+mintSpread]. Set by [createCore] after a lost mint; 0 or 1
	// picks max+1.
	mintSpread int
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
	return generateID(ctx, deps, entityType, opts.IDPrefix, opts.mintSpread)
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
	for attempt := 1; ; attempt++ {
		e, warnings, err := createCoreOnce(ctx, deps, entityType, opts)
		// A generated ID is minted from a scan, so a concurrent create of the
		// same type can take it first. The store's create is atomic on the ID,
		// so the loser lands here and re-mints against the new state. A
		// caller-supplied ID is the caller's choice: its collision is final.
		if opts.ID == "" && errors.Is(err, ErrEntityAlreadyExists) && attempt < idMintAttempts {
			// Every contender re-reads the same max, so a sequential re-mint
			// of max+1 lets only one of them win per round. Spreading the
			// retry over a window that doubles each round resolves a burst in
			// a few rounds, at the cost of a gap in the numbering, and only
			// when creates actually collided.
			opts.mintSpread = min(1<<attempt, maxMintSpread)
			continue
		}
		return e, warnings, err
	}
}

// idMintAttempts bounds [createCore]'s retry on a generated-ID collision.
// Each loss means another create of the same type committed in between.
// With the doubling spread, 10 rounds absorb a burst far larger than the
// write paths produce (the webhook admits 8 in flight).
const idMintAttempts = 10

// maxMintSpread caps the retry window, so a collision never skips more than
// this many sequential numbers.
const maxMintSpread = 64

// createCoreOnce is one attempt of [createCore].
func createCoreOnce(
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
	if enforceErr := deps.Transitions.EnforceCreate(ctx, e); enforceErr != nil {
		return nil, nil, enforceErr
	}

	// A create must never fall through to an update — that would
	// overwrite a colliding entity (a racing create, or a stale-scan
	// duplicate ID). Write with a direct CreateEntity and surface a
	// conflict as ErrEntityAlreadyExists. Every write path is now
	// create-XOR-update by resolved intent; there is no create-then-
	// update-on-conflict fallback anywhere (BUG-ZWTDH9).
	//
	// `unique: true` natural keys are enforced atomically with the write.
	// excludeSelfID is empty: on create there is no prior version of this
	// entity to exclude.
	err = writeWithUniqueCheck(ctx, deps, e, "", func(st store.Store) error {
		return st.CreateEntity(ctx, e)
	})
	if err != nil {
		// A derived unique-property index (pgstore, TKT-3Q0GP1) rejects a
		// duplicate PROPERTY value; surface it as the same 422 the pre-write
		// scan does. Check this BEFORE the ErrConflict branch: UniquePropertyError
		// satisfies errors.Is(_, ErrConflict) too, and the ID-collision message
		// would be wrong for a property duplicate.
		if ok, mapped := mapUniquePropertyConflict(err); ok {
			return nil, nil, mapped
		}
		var invalid *ValidationError
		if errors.As(err, &invalid) {
			return nil, nil, err
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
		// Only a declared default fills an empty status. Without one, an
		// explicit "" is dropped rather than stored (BUG-ZD4PIN).
		if status := entityDef.GetDefaultStatus(deps.Meta); status != "" {
			e.SetString("status", status)
		} else if v, ok := e.Properties["status"]; ok && v == "" {
			delete(e.Properties, "status")
		}
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
// prefix is non-empty it overrides the metamodel-default prefix. spread is
// [createCoreOpts.mintSpread]; random short IDs ignore it.
func generateID(ctx context.Context, deps Deps, entityType, prefix string, spread int) (string, error) {
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
	if spread > 1 {
		//nolint:gosec // G404: spreading contenders over a window, not a secret
		skip := rand.IntN(spread)
		return entity.GenerateSequentialID(existingIDs, prefix, skip), nil
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
// AllFaces, then deduped by bare id. Without it the scan sees only
// zero-coordinate rows, which a type declaring faces has none of — so the
// generator saw an EMPTY id set for such a type and minted the same id for
// every entity of it (BUG-HC6I2T). Counting a family once is what the query
// used to get from the zero coordinate; with no face privileged, dedup is
// what provides it.
func collectAllIDs(ctx context.Context, st store.Store) ([]string, error) {
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	for e, err := range st.ListEntities(ctx, store.EntityQuery{Faces: store.AllFaces()}) {
		if err != nil {
			return nil, err
		}
		if _, dup := seen[e.ID]; dup {
			continue
		}
		seen[e.ID] = struct{}{}
		ids = append(ids, e.ID)
	}
	// A soft-deleted id is still taken: the store refuses to create it until
	// the purge, so the generator must not hand it out.
	if sd, ok := st.(store.SoftDeleteProvider); ok {
		marked, err := sd.SoftDelete().ListMarked(ctx)
		if err != nil {
			return nil, err
		}
		for _, me := range marked {
			if _, dup := seen[me.ID]; !dup {
				seen[me.ID] = struct{}{}
				ids = append(ids, me.ID)
			}
		}
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

// findExistingRelationTarget locates an existing target entity of the given
// type that is the target of a relationType edge from source. Returns nil if
// none exists.
//
// The edge is looked up where the cascade would write it: on source's face
// for a `scope: content` type, on the identity tail otherwise (see
// cascadeTail). A faced trigger's draft must not find the checklist
// its published face already owns.
//
// The target is an entity, so it is found by family: a faced target stores
// no zero-face row, and reading that row missed it (BUG-J3PBFN). The first
// stored face stands for the family; the runner reads only its id and type.
func findExistingRelationTarget(
	ctx context.Context, d Deps, source entity.Ref, relationType, targetType string,
) *entity.Entity {
	tail := d.cascadeTail(relationType, source.Face)
	for rel, err := range d.Store.ListRelations(ctx, store.RelationQuery{
		EntityID:  source.ID,
		Direction: store.DirectionOutgoing,
		Type:      relationType,
		FromFace:  &tail,
	}) {
		if err != nil {
			continue
		}
		family, fErr := familyRows(ctx, d.Store, rel.To)
		if fErr != nil || len(family) == 0 {
			continue
		}
		if family[0].Type == targetType {
			return family[0]
		}
	}
	return nil
}

// cascadeTail is the tail a cascade-written edge of relType carries when its
// source is the row at face: that face for a `scope: content` type, the zero
// (identity) tail otherwise. An unknown relation type is treated as identity;
// the metamodel check that follows rejects it.
func (d Deps) cascadeTail(relType string, face entity.Face) entity.Face {
	if d.isIdentityRelation(relType) {
		return ""
	}
	return face
}

// isIdentityRelation reports whether relType's edges attach to the entity as
// such (`scope: identity`, the default) rather than to one face. An unknown
// type counts as identity.
func (d Deps) isIdentityRelation(relType string) bool {
	def, ok := d.Meta.GetRelationDef(relType)
	return !ok || def.Scope.IsIdentity()
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
		if !face.IsImplicit() {
			return fmt.Errorf("%w: %s declares no faces, so %q names nothing",
				ErrFaceNotDeclared, entityType, face)
		}
		return nil
	}
	if face.IsImplicit() {
		return fmt.Errorf("%w: %s declares %s", ErrFaceRequired,
			entityType, strings.Join(metamodel.FaceOrderOf(d.Meta, entityType), ", "))
	}
	if _, declared := def.Faces[face.String()]; !declared {
		return fmt.Errorf("%w: %s declares %s, not %q", ErrFaceNotDeclared,
			entityType, strings.Join(metamodel.FaceOrderOf(d.Meta, entityType), ", "), face)
	}
	return nil
}

// requireRelationFaceFor rejects a source face that the relation type or the
// source entity type cannot carry, and a missing one where the edge belongs
// to a face. It is the relation-side counterpart of requireCreateFaceFor.
//
// It guards the manager write path only. The cascade host writes relations
// straight to the store (cascadehost.go WriteRelation), so it does not pass
// through here. That is safe because the host picks the tail itself: the
// zero tail for an identity-scoped edge, and for a content edge the face of
// the trigger row, which exists and so is declared (Deps.cascadeTail).
//
//   - `scope: identity` edges attach to the entity as such, so they have no
//     per-face existence and the only valid tail is the zero face. Accepting
//     a named one would store a coordinate the readers never query — the edge
//     exists and nothing returns it (reads filter on the tail), so a caller's
//     "does this link exist" check answers false and re-creates it forever.
//   - `scope: content` edges belong to one face of the source, so a named tail
//     must be one the source type declares, and a faceless source must not
//     name one at all.
//   - A `scope: content` edge from a faced source must name the face. A zero
//     tail there would belong to no face, so no world would show it as any
//     face's content. Every client resolves the face before it gets here:
//     the HTTP API, MCP, the command line and CalDAV from the address, Lua
//     from `opts.face`.
//
// It runs on create and update. Delete does not call it, so an edge stored
// at the zero tail before this rule can still be removed.
//
// Lives HERE rather than in each binding because the ACL is not a backstop
// for it: Manager.authorizeAndAudit returns early under `bypassACL`, so an
// elevated caller reaches the store with no grant check at all. A check below
// that short-circuit covers the elevated path, and every future client (MCP,
// CLI, caldav — see TKT-2RQMV4) inherits it the way entity creates already
// inherit requireCreateFaceFor.
//
// fromType is best-effort at the call sites (empty when the source does not
// exist, mirroring the authorization subject), so a missing source is
// validated on the relation scope alone rather than refused here; the
// peer-existence checks that follow report it. A source that exists but whose
// type the schema no longer declares has no faces, so a named tail on it is
// refused. That applies to updates too: such an edge can be deleted, not
// rewritten.
//
// Nil: returned for a zero face on an identity-scoped type, which is the
// overwhelmingly common case.
func (d Deps) requireRelationFaceFor(relType, fromType string, face entity.Face) error {
	if err := validTail(face); err != nil {
		return err
	}
	relDef, ok := d.Meta.GetRelationDef(relType)
	if !ok {
		// Unknown relation type: ValidateRelation reports it with a better
		// message than a face complaint would.
		return nil
	}
	// Branch through IsIdentity/IsContent, never by comparing to a constant:
	// identity scope has two spellings ("" and "identity").
	if relDef.Scope.IsIdentity() {
		if !face.IsImplicit() {
			return fmt.Errorf("%w: relation %s is scope: identity, so it attaches to the "+
				"entity rather than to %q", ErrFaceNotDeclared, relType, face)
		}
		return nil
	}
	if fromType == "" {
		return nil
	}
	def, defOK := d.Meta.GetEntityDef(fromType)
	if !defOK {
		// A source whose type the schema no longer declares has no faces
		// to name. The zero tail stays valid, as for a faceless type.
		if !face.IsImplicit() {
			return fmt.Errorf("%w: source type %s is not declared, so %q names nothing",
				ErrFaceNotDeclared, fromType, face)
		}
		return nil
	}
	if len(def.Faces) == 0 {
		if !face.IsImplicit() {
			return fmt.Errorf("%w: source type %s declares no faces, so %q names nothing",
				ErrFaceNotDeclared, fromType, face)
		}
		return nil
	}
	if face.IsImplicit() {
		return fmt.Errorf("%w: relation %s, source type %s declares %s", ErrRelationFaceRequired,
			relType, fromType, strings.Join(metamodel.FaceOrderOf(d.Meta, fromType), ", "))
	}
	if _, declared := def.Faces[face.String()]; !declared {
		return fmt.Errorf("%w: source type %s declares %s, not %q", ErrFaceNotDeclared,
			fromType, strings.Join(metamodel.FaceOrderOf(d.Meta, fromType), ", "), face)
	}
	return nil
}

// entityFamily is the entity-level view of one id: its type and the faces it
// stores. It answers the questions every face of a family answers the same
// way, which is all a relation endpoint needs: a relation attaches to the
// ENTITY, and heads are entity-level.
type entityFamily struct {
	id    string
	typ   string
	faces []entity.Face
}

// lookupFamily reads the family of the entity ref names, from headers only.
// ref may be a bare id or the fused `ID@face` form; either way the family of
// the id is returned, because its type and faces are what the callers need.
//
// Reading the zero-face row instead (Store.GetEntity) found nothing for a
// type that declares faces, so every relation touching a faced entity
// reported it missing (BUG-HC6I2T, BUG-J3PBFN).
//
// IDs-scoped, never a full scan: this runs on the relation write path, once
// per endpoint, and an unbounded scan there is the per-row lookup the
// collection-read rules exist to prevent.
//
// FAILS CLOSED. A missing family is [store.ErrNotFound]; any other error is
// returned as-is. Reporting a transient backend error as "missing" would skip
// the ACL check in every caller's not-found branch (existence is itself a
// secret), the defect TestRename_FailsClosedOnNonNotFoundFetchError pins.
func lookupFamily(ctx context.Context, st store.EntityLister, ref string) (entityFamily, error) {
	id := ref
	if base, _, err := entity.ParseStateRef(ref); err == nil {
		id = base
	}
	fam := entityFamily{id: id}
	headers, err := store.FamilyHeaders(ctx, st, id)
	if err != nil {
		return entityFamily{}, err
	}
	for _, h := range headers {
		if fam.typ == "" {
			fam.typ = h.Type
		}
		fam.faces = append(fam.faces, h.Face)
	}
	if len(fam.faces) == 0 {
		return entityFamily{}, store.ErrNotFound
	}
	slices.Sort(fam.faces)
	return fam, nil
}

// validTail refuses a relation tail that is not a face name. A tail reaches
// the store as part of the relation key, so text that no face could carry
// must stop here rather than become a stored coordinate (TKT-7IZHP0). The
// zero tail is the implicit face and always valid.
func validTail(face entity.Face) error {
	if face.IsImplicit() {
		return nil
	}
	if _, err := entity.ParseFace(face.String()); err != nil {
		return fmt.Errorf("%w: relation tail %q is not a valid face name: %w", ErrFaceNotDeclared, face, err)
	}
	return nil
}

// relationWriteSubject builds the authorization subject for a relation write from
// source, applying ruling D4 (TKT-KQXVF7). An edge with a named tail is
// authorized at that face. A zero-tailed edge from a faced source belongs to
// the entity as a whole, so it is authorized on every face the source TYPE
// declares. That holds for a `scope: content` edge at the zero tail too: it
// is the identity coordinate, so a bare-type grant, which covers only the
// zero face, must not be enough for it either. A missing source leaves the
// type empty, which matches no grant.
//
// The faces come from the schema, never from the faces this entity stores.
// Deciding per stored face would consult faces the caller cannot read, and
// the answer (and the face a denial names) would disclose that a hidden face
// exists. The declared set is a superset of the stored one, so this asks at
// least what the stored set did.
func relationWriteSubject(
	meta *metamodel.Metamodel, relType string, source entityFamily, from string, tail entity.Face,
) acl.RelationSubject {
	s := acl.RelationSubject{Type: relType, FromType: source.typ, FromID: from, FromFace: tail}
	if tail.IsImplicit() {
		s.FamilyFaces = declaredFaces(meta, source.typ)
	}
	return s
}

// declaredFaces returns the faces typ declares, in declaration order; nil for
// a faceless or unknown type.
func declaredFaces(meta *metamodel.Metamodel, typ string) []entity.Face {
	if typ == "" {
		return nil
	}
	names := metamodel.FaceOrderOf(meta, typ)
	if len(names) == 0 {
		return nil
	}
	faces := make([]entity.Face, len(names))
	for i, n := range names {
		faces[i] = entity.Face(n)
	}
	return faces
}

// RelationCreateRequest is the authorization request [Manager.CreateRelation]
// runs for an edge from fromID (type fromType) with the given tail. It is
// exported so a caller that answers "may this principal create the edge?"
// ahead of the write asks the question the write will ask, instead of a copy
// of it. It reads no store: the answer depends on the schema and the grants.
func RelationCreateRequest(
	meta *metamodel.Metamodel, relType, fromType, fromID string, tail entity.Face,
) acl.WriteRequest {
	return relationRequest(meta, acl.OpCreate, relType, fromType, fromID, tail)
}

// RelationUpdateRequest is the authorization request [Manager.UpdateRelation]
// runs, exported for the reason [RelationCreateRequest] is.
func RelationUpdateRequest(
	meta *metamodel.Metamodel, relType, fromType, fromID string, tail entity.Face,
) acl.WriteRequest {
	return relationRequest(meta, acl.OpUpdate, relType, fromType, fromID, tail)
}

// RelationDeleteRequest is the authorization request [Manager.DeleteRelation]
// runs, exported for the reason [RelationCreateRequest] is.
func RelationDeleteRequest(
	meta *metamodel.Metamodel, relType, fromType, fromID string, tail entity.Face,
) acl.WriteRequest {
	return relationRequest(meta, acl.OpDelete, relType, fromType, fromID, tail)
}

func relationRequest(
	meta *metamodel.Metamodel, op acl.Op, relType, fromType, fromID string, tail entity.Face,
) acl.WriteRequest {
	source := entityFamily{id: fromID, typ: fromType}
	return acl.WriteRequest{Op: op, Subject: relationWriteSubject(meta, relType, source, fromID, tail)}
}

// getEntityByRef resolves an entity ADDRESS — either a bare id or the fused
// boundary form `ID@face` — to the row it names.
//
// A bare id names the implicit face of a faceless type. A type declaring
// `faces:` stores no row there (BUG-HC6I2T removed the privileged face), so a
// bare id on such a type reports not found; the caller must name the face.
// An unparseable ref names no row at all.
//
// Fails closed on a non-not-found error, for the same reason [lookupFamily]
// does: a caller's not-found branch typically returns before authorizing, so
// a transient store error reported as "absent" would skip an ACL check.
//
// Nil: never returned with a nil error.
func (m *Manager) getEntityByRef(ctx context.Context, ref string) (*entity.Entity, error) {
	r, err := entity.ParseRef(ref)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", store.ErrNotFound, ref)
	}
	return m.deps.Store.GetEntity(ctx, entity.Ref{ID: r.ID, Face: r.Face})
}
