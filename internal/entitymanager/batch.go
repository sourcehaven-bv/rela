package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// MaxBatchOperations is the most entities plus relations one [CreateBatch]
// call creates (DEC-ODU65KR). It is fixed, not configurable: one batch
// holds the deployment-wide write lock for all of its writes.
const MaxBatchOperations = 100

// Batch is a create of several entities and the relations between them
// that [CreateBatch] commits in one store transaction.
type Batch struct {
	Entities  []BatchEntity
	Relations []BatchRelation
}

// BatchEntity is one entity create of a [Batch]. Entity carries the type,
// the properties and the content; Options carries the address (id, prefix,
// face) and the template variant, as for [Manager.CreateEntity]. Every
// property in Entity.Properties is the caller's and passes the field gate.
type BatchEntity struct {
	Entity  *entity.Entity
	Options entity.CreateOptions
}

// BatchRelation is one relation create of a [Batch].
type BatchRelation struct {
	From BatchEnd
	Type string
	To   BatchEnd
}

// BatchEnd is one end of a [BatchRelation]: an existing entity when ID is
// set, otherwise the entity at Index in [Batch.Entities].
//
// Face is the tail an edge from an existing entity leaves. It is used for a
// content-scoped relation only, as on [Manager.CreateRelation]. An edge
// from a batch entity leaves the face that entity is created at. A head
// names an entity, not a face, so a To end with a Face is refused.
type BatchEnd struct {
	Index int
	ID    string
	Face  entity.Face
}

// local reports whether the end names an entity of the batch.
func (e BatchEnd) local() bool { return e.ID == "" }

// BatchResult is the outcome of [CreateBatch], in the order of the batch.
type BatchResult struct {
	// Entities holds one result per [Batch.Entities] entry. After a
	// follow-up failure the result holds what was done before it.
	Entities []*entity.CreateResult
	// Relations holds one written relation per [Batch.Relations] entry.
	Relations []*entity.Relation
	// FollowUpErrs holds, per [Batch.Entities] entry, the failure of the
	// steps after the commit: the automations and the cascade. Nil entries
	// mean they succeeded. The entities and relations stay written.
	FollowUpErrs []error
}

// Batch operation kinds, as [BatchOpError.Kind] names them.
const (
	BatchOpEntity   = "entity"
	BatchOpRelation = "relation"
)

// BatchOpError is a [CreateBatch] failure of one operation: Kind is
// [BatchOpEntity] or [BatchOpRelation], Index its position in
// [Batch.Entities] or [Batch.Relations]. It wraps the refusal, so
// errors.Is, errors.As and [ClassifyCreateError] see through it.
type BatchOpError struct {
	Kind  string
	Index int
	// End is the end of a relation that does not exist or that the caller
	// cannot read, [BatchEndFrom] or [BatchEndTo]; empty for any other
	// refusal. The two cases are not told apart.
	End string
	Err error
}

// Relation ends, as [BatchOpError.End] names them.
const (
	BatchEndFrom = "from"
	BatchEndTo   = "to"
)

func (e *BatchOpError) Error() string {
	return fmt.Sprintf("%s %d: %v", e.Kind, e.Index, e.Err)
}

func (e *BatchOpError) Unwrap() error { return e.Err }

// ErrBatchInvalid is a [Batch] whose shape is wrong: empty, over
// [MaxBatchOperations], a nil entity, an end that names no batch entity, or
// a head with a face. The [BatchOpError] wrapping it names the operation.
var ErrBatchInvalid = errors.New("invalid batch")

// BatchKeptError is a [CreateBatch] failure after which some writes stay:
// the transaction failed on a store that cannot roll back. The kept rows
// were audited and their automations ran. Err is the failure, usually a
// *[BatchOpError].
type BatchKeptError struct {
	Entities  []*entity.Entity
	Relations []*entity.Relation
	Err       error
}

func (e *BatchKeptError) Error() string {
	return fmt.Sprintf("%d entities and %d relations were created, but the rest of the batch failed: %v",
		len(e.Entities), len(e.Relations), e.Err)
}

func (e *BatchKeptError) Unwrap() error { return e.Err }

// CreateBatch creates every entity and relation of b in one
// [store.Store.Tx]. On a store with a real transaction (postgres, sqlite)
// they commit together or not at all. On the filesystem and memory stores
// the transaction only excludes other writers, so a failure leaves the
// writes made before it; the caller decides whether to offer the batch
// there at all.
//
// Every operation is checked as the caller before anything is written,
// with the checks [Manager.CreateEntity] and [Manager.CreateRelation] make:
// per entity the face, the create ACL, the field gate (through a
// [FieldGated] handle), the file and external-ref refusals and the custom
// id; per relation the face, the relation ACL, the existing ends' read
// gate and the relation-type tuple. An existing end the caller cannot read
// is refused exactly like a missing one. An end created by the batch is
// judged from its planned type and face and is never looked up, as
// [CreateLinked] does. The relation ACL runs before the new entity has an
// id; that is sound because a relation write is decided from the source
// type and the caller's global roles only (pinned by
// acl.TestRelationWrite_DecisionIgnoresSourceID). The first refusal ends
// the batch with nothing written.
//
// In the transaction every write goes through the view. First every
// relation with an existing end is checked again on the view (the end
// still exists with an allowed type, its bounds and ownership rules still
// admit the edge), so a write that landed between the checks and the
// transaction refuses the batch rather than committing a dangling edge.
// Then the entities in order, with generated ids minted under the store's write lock so the
// view sees the batch's own earlier ids, then the relations, with their
// cardinality bounds and ownership rules counted on the view.
//
// After the commit: the audit record and alias notification of every
// entity, the audit record of every relation, and then each entity's
// on-create automations and cascade, in order, as [Manager.CreateEntity]
// runs them. Their failure is [BatchResult.FollowUpErrs], not an error,
// because the batch is committed. Nothing is audited for a batch the
// store rolled back, which the store reports through [store.TxRollsBack];
// on a store that cannot roll back the rows that stay are audited and the
// error is a *[BatchKeptError].
//
// A failure of one operation is a *[BatchOpError] naming it.
//
// A package function rather than a Manager method: Manager is at its
// method cap.
func CreateBatch(ctx context.Context, m *Manager, b Batch) (*BatchResult, error) {
	ctx = withStoreAttribution(ctx)
	t, err := prepareBatch(ctx, m, b)
	if err != nil {
		return nil, err
	}
	err = m.deps.Store.Tx(ctx, func(view store.Store) error {
		return t.run(store.ContextInTx(ctx), view)
	})
	if err != nil {
		return nil, t.failed(ctx, err)
	}
	return t.finish(ctx), nil
}

// batchCreate is one [CreateBatch] call: what it prepared before the
// transaction, and what the transaction wrote.
type batchCreate struct {
	m *Manager
	b Batch
	// keys and rels are the relations, with a batch end's id blank until
	// that entity is written.
	keys []entity.RelationKey
	rels []*entity.Relation

	created  []*entity.Entity
	warnings [][]entity.Warning
	written  []*entity.Relation
}

// prepareBatch checks the shape of b and runs every check that needs no
// transaction, in batch order: the entities, then the relations.
func prepareBatch(ctx context.Context, m *Manager, b Batch) (*batchCreate, error) {
	if err := checkBatchShape(b); err != nil {
		return nil, err
	}
	for i, be := range b.Entities {
		if err := prepareCreate(ctx, m, be.Entity, be.Options, be.Entity.Properties); err != nil {
			return nil, &BatchOpError{Kind: BatchOpEntity, Index: i, Err: err}
		}
	}
	t := &batchCreate{
		m: m, b: b,
		keys:     make([]entity.RelationKey, len(b.Relations)),
		rels:     make([]*entity.Relation, len(b.Relations)),
		created:  make([]*entity.Entity, len(b.Entities)),
		warnings: make([][]entity.Warning, len(b.Entities)),
		written:  make([]*entity.Relation, len(b.Relations)),
	}
	for j, br := range b.Relations {
		key, rel, end, err := prepareBatchRelation(ctx, m, b, br)
		if err != nil {
			return nil, &BatchOpError{Kind: BatchOpRelation, Index: j, End: end, Err: err}
		}
		t.keys[j], t.rels[j] = key, rel
	}
	return t, nil
}

// checkBatchShape refuses a batch the engine cannot run, before any check
// that could audit a denial.
func checkBatchShape(b Batch) error {
	n := len(b.Entities) + len(b.Relations)
	if n == 0 {
		return fmt.Errorf("%w: a batch creates at least one entity or relation", ErrBatchInvalid)
	}
	if n > MaxBatchOperations {
		return fmt.Errorf("%w: %d operations, at most %d", ErrBatchInvalid, n, MaxBatchOperations)
	}
	for i, be := range b.Entities {
		if be.Entity == nil {
			return &BatchOpError{Kind: BatchOpEntity, Index: i, Err: fmt.Errorf("%w: entity is nil", ErrBatchInvalid)}
		}
	}
	for j, br := range b.Relations {
		if err := checkBatchEnds(b, br); err != nil {
			return &BatchOpError{Kind: BatchOpRelation, Index: j, Err: err}
		}
	}
	return nil
}

// checkBatchEnds refuses an end that names no batch entity, and a head
// with a face.
func checkBatchEnds(b Batch, br BatchRelation) error {
	for _, end := range []BatchEnd{br.From, br.To} {
		if end.local() && (end.Index < 0 || end.Index >= len(b.Entities)) {
			return fmt.Errorf("%w: entity index %d is not in the batch", ErrBatchInvalid, end.Index)
		}
	}
	if br.To.Face != "" {
		return fmt.Errorf("%w: a relation head names an entity, not a face", ErrBatchInvalid)
	}
	if br.From.local() && br.From.Face != "" {
		return fmt.Errorf("%w: the tail of an edge from a batch entity is that entity's face", ErrBatchInvalid)
	}
	return nil
}

// plannedFamily is the family the batch entity at i will have once
// written: its type and face, and its id when the caller chose one.
func plannedFamily(b Batch, i int) entityFamily {
	be := b.Entities[i]
	return entityFamily{id: be.Options.ID, typ: be.Entity.Type, faces: []entity.Face{be.Options.Face}}
}

// prepareBatchRelation runs the checks of one relation before any batch
// entity has an id. It returns the key and relation with a batch end's id
// still blank, unless the caller chose that id, and on a missing or hidden
// end which end that is.
func prepareBatchRelation(
	ctx context.Context, m *Manager, b Batch, br BatchRelation,
) (entity.RelationKey, *entity.Relation, string, error) {
	key := entity.RelationKey{Type: br.Type}
	var source entityFamily
	var srcErr error
	if br.From.local() {
		source = plannedFamily(b, br.From.Index)
		key.From = source.id
		key.FromFace = relationTail(m, br.Type, b.Entities[br.From.Index].Options.Face)
	} else {
		// The named tail as given: requireRelationFaceFor refuses a face
		// on an identity relation, as a single create does.
		key.From = br.From.ID
		key.FromFace = br.From.Face
		// A source the caller cannot see is looked up as a missing one.
		source, srcErr = lookupSource(ctx, m, key.From, key.FromFace)
	}
	var created *entityFamily
	if br.To.local() {
		target := plannedFamily(b, br.To.Index)
		created = &target
		key.To = target.id
	} else {
		key.To = br.To.ID
	}
	rel, err := prepareRelationFrom(ctx, m, key, entity.RelationOptions{}, source, srcErr, created)
	var end string
	if errors.Is(err, ErrEntityNotFound) {
		// Only an existing end is looked up, and the source first.
		end = BatchEndTo
		if srcErr != nil {
			end = BatchEndFrom
		}
	}
	return key, rel, end, err
}

// run is the transaction: the entities, then the relations, all on view.
// txCtx is marked with [store.ContextInTx].
func (t *batchCreate) run(txCtx context.Context, view store.Store) error {
	if err := t.recheckEnds(txCtx, view); err != nil {
		return err
	}
	deps := t.m.deps
	deps.Store = view
	for i, be := range t.b.Entities {
		c, w, err := createCore(txCtx, deps, be.Entity.Type, createCoreArgs(be.Entity, be.Options))
		if err != nil {
			return &BatchOpError{Kind: BatchOpEntity, Index: i, Err: err}
		}
		t.created[i], t.warnings[i] = c, w
	}
	for j, br := range t.b.Relations {
		key, rel := t.keys[j], t.rels[j]
		if br.From.local() {
			key.From, rel.From = t.created[br.From.Index].ID, t.created[br.From.Index].ID
		}
		if br.To.local() {
			key.To, rel.To = t.created[br.To.Index].ID, t.created[br.To.Index].ID
		}
		if err := writeLinkedRelation(txCtx, view, t.m, key, rel); err != nil {
			return &BatchOpError{Kind: BatchOpRelation, Index: j, Err: err}
		}
		t.written[j] = rel
	}
	return nil
}

// recheckEnds runs, on the view and before any write, the checks of every
// relation with an existing end that a write between the preparation and
// the transaction could have changed: the existing ends still exist with a
// type the relation allows, and the cardinality bound and ownership rules
// of the existing ends still admit the edge. A refusal writes nothing on
// any store. Only existence, the tail face and type are read: the read gate
// ran before the transaction, and a deleted end is reported like a missing
// one.
func (t *batchCreate) recheckEnds(ctx context.Context, view store.Store) error {
	v := metamodel.NewValidator(t.m.deps.Meta)
	for j, br := range t.b.Relations {
		if br.From.local() && br.To.local() {
			continue
		}
		fromType, err := t.endType(ctx, view, br.From, "source")
		if err != nil {
			return &BatchOpError{Kind: BatchOpRelation, Index: j, End: endOf(err, BatchEndFrom), Err: err}
		}
		toType, err := t.endType(ctx, view, br.To, "target")
		if err != nil {
			return &BatchOpError{Kind: BatchOpRelation, Index: j, End: endOf(err, BatchEndTo), Err: err}
		}
		if vErr := v.ValidateRelation(br.Type, fromType, toType); vErr != nil {
			return &BatchOpError{Kind: BatchOpRelation, Index: j, Err: &InvalidRelationError{
				Key: t.keys[j], err: fmt.Errorf("invalid relation: %w", vErr),
			}}
		}
		if err := checkLinkedRelation(ctx, view, t.m, t.keys[j]); err != nil {
			return &BatchOpError{Kind: BatchOpRelation, Index: j, Err: err}
		}
	}
	return nil
}

// endOf is end when err reports a missing end, and empty otherwise.
func endOf(err error, end string) string {
	if errors.Is(err, ErrEntityNotFound) {
		return end
	}
	return ""
}

// endType is the type of one relation end: the planned type of a batch
// entity, or the stored type of an existing one, read on view.
func (t *batchCreate) endType(ctx context.Context, view store.Store, end BatchEnd, side string) (string, error) {
	if end.local() {
		return t.b.Entities[end.Index].Entity.Type, nil
	}
	fam, err := lookupFamily(ctx, view, end.ID)
	if err != nil {
		return "", missingEndpoint(side, end.ID, err)
	}
	if end.Face != "" && !slices.Contains(fam.faces, end.Face) {
		// The tail face of a content-scoped edge was deleted since the
		// preparation: the edge would dangle from it.
		return "", missingEndpoint(side, end.ID, store.ErrNotFound)
	}
	return fam.typ, nil
}

// finish runs the steps after the commit: every audit record and alias
// notification first, so the committed batch is recorded whole before any
// automation runs, then the follow-ups per entity.
func (t *batchCreate) finish(ctx context.Context) *BatchResult {
	for _, c := range t.created {
		recordCreate(ctx, t.m, c)
	}
	for _, r := range t.written {
		t.m.recordRelationAudit(ctx, audit.OpCreateRelation, r, "created")
	}
	out := &BatchResult{
		Entities:     make([]*entity.CreateResult, len(t.created)),
		Relations:    t.written,
		FollowUpErrs: make([]error, len(t.created)),
	}
	for i, c := range t.created {
		res := &entity.CreateResult{Entity: c, Warnings: t.warnings[i]}
		out.Entities[i], out.FollowUpErrs[i] = createFollowUps(ctx, t.m, res, t.b.Entities[i].Options.SkipAutomation)
	}
	return out
}

// failed handles a failed transaction. On a store that rolled back nothing
// stays and err is returned. On a store that cannot roll back, the rows
// that stayed are audited and their follow-ups run, so no row exists
// without its audit record, and the error is a *[BatchKeptError].
//
// Which store rolls back is asked of the store, never read back from it: a
// rolled-back generated id is free again, and a create racing the read-back
// could take it and be audited as this batch's.
//
// Two cases this does not cover. A commit that succeeded but reported an
// error (a lost acknowledgement on postgres) leaves rows with no audit
// record; telling that apart would need the read-back this refuses. And a
// batch run inside an open transaction joins it, so its rows go only when
// the caller's transaction rolls back too; CreateBatch has no such caller.
func (t *batchCreate) failed(ctx context.Context, err error) error {
	if store.TxRollsBack(t.m.deps.Store) {
		return err
	}
	kept := &BatchKeptError{Err: err}
	for i, c := range t.created {
		if c == nil {
			continue
		}
		if _, gErr := t.m.deps.Store.GetEntity(ctx, c.Ref()); gErr != nil {
			continue
		}
		recordCreate(ctx, t.m, c)
		kept.Entities = append(kept.Entities, c)
		res := &entity.CreateResult{Entity: c, Warnings: t.warnings[i]}
		if _, fErr := createFollowUps(ctx, t.m, res, t.b.Entities[i].Options.SkipAutomation); fErr != nil {
			slog.WarnContext(ctx, "entitymanager: CreateBatch: a step after a kept create failed",
				"entity", c.ID, "error", fErr)
		}
	}
	for _, r := range t.written {
		if r == nil {
			continue
		}
		if _, gErr := t.m.deps.Store.GetRelation(ctx, r.Identity()); gErr == nil {
			t.m.recordRelationAudit(ctx, audit.OpCreateRelation, r, "created")
			kept.Relations = append(kept.Relations, r)
		}
	}
	if len(kept.Entities) == 0 && len(kept.Relations) == 0 {
		return err
	}
	return kept
}
