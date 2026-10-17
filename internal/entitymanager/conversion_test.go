package entitymanager_test

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// conversionSchema is the unfaced fixture: a note converts to a task, and
// each relation kind a conversion can meet is declared: kept, dropped and
// remapped. vault is the type the ACL tests hide.
const conversionSchema = `version: "1"
entities:
  note:
    label: Note
    id_prefix: "NOTE-"
    id_type: manual
    properties:
      title: {type: string, required: true}
      body: {type: string}
  task:
    label: Task
    id_prefix: "TASK-"
    id_type: sequential
    properties:
      title: {type: string, required: true}
      description: {type: string}
      code: {type: string, unique: true}
  peer:
    label: Peer
    id_prefix: "PEER-"
    id_type: manual
    properties: {title: {type: string}}
  vault:
    label: Vault
    id_prefix: "VAULT-"
    id_type: manual
    properties: {title: {type: string}}
relations:
  relates-to:
    label: Relates to
    from: [note, task]
    to: [note, task, peer]
  stored-in:
    label: Stored in
    from: [note, task]
    to: [vault]
  watched-by:
    label: Watched by
    from: [note]
    to: [peer, vault]
  followed-by:
    label: Followed by
    from: [task]
    to: [peer, vault]
conversions:
  note-to-task:
    from: note
    to: task
    fields:
      title: "{{source.title}}"
      description: "{{source.body}}"
    relations:
      relates-to: relates-to
      stored-in: drop
      watched-by: followed-by
`

// conversionPolicy lets alice read and write everything but vaults, so an
// edge to a vault is one she cannot see.
const conversionPolicy = `
roles:
  editor:
    read: [note, task, peer]
    create: [note, task, peer]
    update: [note, task, peer]
    delete: [note, task, peer]
    convert: [note]
assignments:
  alice: editor
client_baselines:
  no-task-create:
    applies_to: [no-task-create]
    deny_create: [task]
  task-only:
    applies_to: [task-only]
    create: [task]
`

type conversionFixture struct {
	st    store.Store
	conv  *entitymanager.Converter
	audit *audit.Memory
	hooks *conversionHooks
	mgr   *entitymanager.Manager
}

type conversionOptions struct {
	schema string
	// policy, when set, gates the manager with a declarative ACL.
	policy string
	// visibility replaces the ungated copy reader.
	visibility func(store.Store) entitymanager.CopyReader
	// wrap decorates the store the manager writes through.
	wrap func(store.Store) store.Store
	// fieldGate, when set, gates the caller's values through a field-gated
	// handle, as the data-entry API does.
	fieldGate entitymanager.FieldWriteGate
	// guard replaces the inactive channel guard.
	guard entitymanager.AnchorGuard
}

func newConversionFixture(t *testing.T, opts conversionOptions) conversionFixture {
	t.Helper()
	if opts.schema == "" {
		opts.schema = conversionSchema
	}
	meta, err := metamodel.Parse([]byte(opts.schema))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	transitions, err := statemachine.Compile(meta)
	if err != nil {
		t.Fatalf("compile transitions: %v", err)
	}
	st := memstore.New()
	var gate acl.ACL = acl.NopACL{}
	if opts.policy != "" {
		policy, pErr := acl.LoadPolicyBytes([]byte(opts.policy))
		if pErr != nil {
			t.Fatalf("load policy: %v", pErr)
		}
		decl, dErr := acl.NewDeclarative(policy, aclstore.MustNewGraph(st), st)
		if dErr != nil {
			t.Fatalf("NewDeclarative: %v", dErr)
		}
		gate = decl
	}
	var visibility entitymanager.CopyReader = allowAllCopyVisibility(t, st)
	if opts.visibility != nil {
		visibility = opts.visibility(st)
	}
	managed := store.Store(st)
	if opts.wrap != nil {
		managed = opts.wrap(st)
	}
	var fields entitymanager.FieldWriteGate = entitymanager.AllowAllFieldGate{}
	if opts.fieldGate != nil {
		fields = opts.fieldGate
	}
	hooks := &conversionHooks{}
	auditor := audit.NewMemory()
	var guard entitymanager.AnchorGuard = entitymanager.NopAnchorGuard{}
	if opts.guard != nil {
		guard = opts.guard
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: managed, Meta: meta, Templater: nopTemplater{}, Audit: auditor, ACL: gate,
		Transitions: transitions, FieldGate: fields,
		CopyReadGate: entitymanager.AllowAllCopyReadGate{}, CopyVisibility: visibility,
		AliasRewriter: hooks, VersionRecorder: hooks, RelationVersionRecorder: hooks,
		AnchorGuard: guard,
	})
	if err != nil {
		t.Fatalf("manager: %v", err)
	}
	if opts.fieldGate != nil {
		mgr = entitymanager.FieldGated(mgr)
	}
	conv, err := entitymanager.NewConverter(mgr)
	if err != nil {
		t.Fatalf("converter: %v", err)
	}
	return conversionFixture{st: st, conv: conv, audit: auditor, hooks: hooks, mgr: mgr}
}

// seed writes e straight to the store.
func (f conversionFixture) seed(t *testing.T, id, typ string, props map[string]any) {
	t.Helper()
	e := entity.New(id, typ)
	maps.Copy(e.Properties, props)
	if err := f.st.CreateEntity(context.Background(), e); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func (f conversionFixture) link(t *testing.T, from, relType, to string) {
	t.Helper()
	key := entity.RelationKey{From: from, Type: relType, To: to}
	if _, err := f.st.CreateRelation(context.Background(), key, &store.RelationData{}); err != nil {
		t.Fatalf("link %s: %v", key, err)
	}
}

func (f conversionFixture) hasRelation(from, relType, to string) bool {
	_, err := f.st.GetRelation(context.Background(), entity.RelationKey{From: from, Type: relType, To: to})
	return err == nil
}

func (f conversionFixture) auditOps(op string) []audit.Record {
	var out []audit.Record
	for _, r := range f.audit.Records() {
		if r.Op == op {
			out = append(out, r)
		}
	}
	return out
}

func (f conversionFixture) asAlice(t *testing.T, principalType string) context.Context {
	t.Helper()
	return entitymanager.BindAs(stampAlice(principalType), t, f.mgr)
}

// stampAlice stamps alice, with principalType as a verified claim when set.
func stampAlice(principalType string) context.Context {
	if principalType == "" {
		return principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	}
	return principal.With(context.Background(), principal.VerifiedFrom(
		"alice", principal.ToolDataEntry, principal.Claims{PrincipalType: principalType}))
}

func TestNewConverter_RejectsNilManager(t *testing.T) {
	t.Parallel()
	if _, err := entitymanager.NewConverter(nil); err == nil {
		t.Fatal("NewConverter(nil) succeeded")
	}
}

func TestConvert_PreservesLineageAndMapsProperties(t *testing.T) {
	t.Parallel()
	f := newConversionFixture(t, conversionOptions{})
	ctx := context.Background()
	f.seed(t, "NOTE-1", "note", map[string]any{"title": "Quick note", "body": "Keep this detail"})
	f.seed(t, "PEER-1", "peer", nil)
	f.link(t, "NOTE-1", "relates-to", "PEER-1")

	offers := f.conv.Offers(ctx, "note", "NOTE-1")
	if len(offers) != 1 || !offers[0].Allowed || !reflect.DeepEqual(offers[0].MappedFields, []string{"description", "title"}) {
		t.Fatalf("offers = %+v", offers)
	}
	req := entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-1"}
	preview, err := f.conv.Preview(ctx, req)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(preview.Required) != 0 || len(preview.RelationsToDrop) != 0 || len(preview.InvalidRelations) != 0 {
		t.Fatalf("preview = %+v", preview)
	}
	if want := []entitymanager.ConversionMove{{}}; !reflect.DeepEqual(preview.Moves, want) {
		t.Errorf("moves = %+v, want %+v", preview.Moves, want)
	}
	result, err := f.conv.Convert(ctx, req)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	got := result.Entity
	if result.OldID != "NOTE-1" || got.ID != "TASK-001" || got.Type != "task" {
		t.Fatalf("result = old %q, entity %+v", result.OldID, got)
	}
	if got.GetString("title") != "Quick note" || got.GetString("description") != "Keep this detail" {
		t.Errorf("mapped properties = %v", got.Properties)
	}
	if _, err := f.st.GetEntity(ctx, entity.Ref{ID: "NOTE-1"}); err == nil {
		t.Error("old id still resolves")
	}
	if !f.hasRelation("TASK-001", "relates-to", "PEER-1") {
		t.Error("kept relation did not follow the rename")
	}
	if !reflect.DeepEqual(f.hooks.renamed, [][2]string{{"NOTE-1", "TASK-001"}}) {
		t.Errorf("alias rewrites = %v", f.hooks.renamed)
	}
	if len(f.hooks.versions) != 1 || f.hooks.versions[0].PrevID != "NOTE-1" || f.hooks.versions[0].Type != "task" {
		t.Errorf("versions = %+v", f.hooks.versions)
	}
	records := f.auditOps(audit.OpConvertEntity)
	if len(records) != 1 || records[0].Before.Type != "note" || records[0].After.Type != "task" {
		t.Errorf("convert audit = %+v", records)
	}
}

func TestConvert_RefusesInvalidRequests(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		req  entitymanager.ConversionRequest
		want error
	}{
		{"unknown conversion", entitymanager.ConversionRequest{Definition: "nope", SourceID: "NOTE-1"},
			entitymanager.ErrUnknownConversion},
		{"absent source", entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-9"},
			entitymanager.ErrEntityNotFound},
		{"override of a mapped field", entitymanager.ConversionRequest{
			Definition: "note-to-task", SourceID: "NOTE-1", Properties: map[string]any{"title": "x"},
		}, entitymanager.ErrConversionInvalid},
		{"unknown target property", entitymanager.ConversionRequest{
			Definition: "note-to-task", SourceID: "NOTE-1", Properties: map[string]any{"bogus": "x"},
		}, entitymanager.ErrConversionInvalid},
		{"source face not stored", entitymanager.ConversionRequest{
			Definition: "note-to-task", SourceID: "NOTE-1", SourceFace: "draft",
		}, entitymanager.ErrConversionInvalid},
		{"target id for a generated type", entitymanager.ConversionRequest{
			Definition: "note-to-task", SourceID: "NOTE-1", TargetID: "TASK-9",
		}, entitymanager.ErrConversionInvalid},
		{"drop of a relation that maps", entitymanager.ConversionRequest{
			Definition: "note-to-task", SourceID: "NOTE-1", DropRelations: []string{"relates-to"},
		}, entitymanager.ErrConversionInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newConversionFixture(t, conversionOptions{})
			f.seed(t, "NOTE-1", "note", map[string]any{"title": "Quick note"})
			if _, err := f.conv.Convert(context.Background(), tc.req); !errors.Is(err, tc.want) {
				t.Fatalf("Convert = %v, want %v", err, tc.want)
			}
			if _, err := f.st.GetEntity(context.Background(), entity.Ref{ID: "NOTE-1"}); err != nil {
				t.Fatalf("refused conversion changed the source: %v", err)
			}
		})
	}
}

func TestConvert_RelationsAreDroppedAndRemappedWithRecords(t *testing.T) {
	t.Parallel()
	f := newConversionFixture(t, conversionOptions{})
	ctx := context.Background()
	f.seed(t, "NOTE-1", "note", map[string]any{"title": "Quick note"})
	f.seed(t, "PEER-1", "peer", nil)
	f.seed(t, "VAULT-1", "vault", nil)
	f.link(t, "NOTE-1", "stored-in", "VAULT-1")
	f.link(t, "NOTE-1", "watched-by", "PEER-1")

	req := entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-1"}
	preview, err := f.conv.Preview(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preview.RelationsToDrop, []string{"stored-in"}) {
		t.Fatalf("preview drops = %v", preview.RelationsToDrop)
	}
	if _, err = f.conv.Convert(ctx, req); !errors.Is(err, entitymanager.ErrConversionInvalid) {
		t.Fatalf("unconfirmed drop = %v, want ErrConversionInvalid", err)
	}
	req.DropRelations = []string{"stored-in"}
	result, err := f.conv.Convert(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	newID := result.Entity.ID
	if result.RelationsDropped != 1 || f.hasRelation(newID, "stored-in", "VAULT-1") {
		t.Errorf("dropped = %d, edge still stored = %v", result.RelationsDropped, f.hasRelation(newID, "stored-in", "VAULT-1"))
	}
	if !f.hasRelation(newID, "followed-by", "PEER-1") || f.hasRelation(newID, "watched-by", "PEER-1") {
		t.Error("watched-by was not remapped to followed-by")
	}

	var deleted []string
	for _, r := range f.auditOps(audit.OpDeleteRelation) {
		deleted = append(deleted, r.Subject.RelationType)
	}
	slices.Sort(deleted)
	if !reflect.DeepEqual(deleted, []string{"stored-in", "watched-by"}) {
		t.Errorf("delete-relation audit = %v", deleted)
	}
	if created := f.auditOps(audit.OpCreateRelation); len(created) != 1 || created[0].Subject.RelationType != "followed-by" {
		t.Errorf("create-relation audit = %+v", created)
	}
	var versions []string
	for _, v := range f.hooks.relVersions {
		if v.Op != store.VersionOpDelete || v.RecordID != fakeLineageID || !strings.HasPrefix(v.TriggeredBy, "convert-entity:") {
			t.Errorf("relation version = %+v", v)
		}
		versions = append(versions, v.Type)
	}
	slices.Sort(versions)
	if !reflect.DeepEqual(versions, []string{"stored-in", "watched-by"}) {
		t.Errorf("relation delete versions = %v", versions)
	}
}

func TestConvert_EnforcesUniqueOnTheTarget(t *testing.T) {
	t.Parallel()
	f := newConversionFixture(t, conversionOptions{})
	f.seed(t, "TASK-500", "task", map[string]any{"title": "Holder", "code": "C-1"})
	f.seed(t, "NOTE-1", "note", map[string]any{"title": "Quick note"})
	_, err := f.conv.Convert(context.Background(), entitymanager.ConversionRequest{
		Definition: "note-to-task", SourceID: "NOTE-1", Properties: map[string]any{"code": "C-1"},
	})
	var verr *entitymanager.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Convert = %v, want a unique violation", err)
	}
	if _, err := f.st.GetEntity(context.Background(), entity.Ref{ID: "NOTE-1"}); err != nil {
		t.Fatalf("refused conversion changed the source: %v", err)
	}
}

// denyFieldGate refuses any create that sets field.
type denyFieldGate struct{ field string }

var errFieldDenied = errors.New("field write denied")

func (g denyFieldGate) CheckFieldWrite(context.Context, *entity.Entity, map[string]any, []string) error {
	return nil
}

func (g denyFieldGate) CheckCreateFieldWrite(_ context.Context, _ *entity.Entity, set map[string]any) error {
	if _, ok := set[g.field]; ok {
		return errFieldDenied
	}
	return nil
}

func TestConvert_GatesTheCallersValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		props map[string]any
		want  error
	}{
		{"denied field supplied", map[string]any{"code": "C-1"}, errFieldDenied},
		// Mapped values are the operator's, so the gate does not judge them.
		{"only mapped values", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newConversionFixture(t, conversionOptions{fieldGate: denyFieldGate{field: "code"}})
			f.seed(t, "NOTE-1", "note", map[string]any{"title": "Quick note", "body": "mapped"})
			_, err := f.conv.Convert(context.Background(), entitymanager.ConversionRequest{
				Definition: "note-to-task", SourceID: "NOTE-1", Properties: tc.props,
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("Convert = %v, want %v", err, tc.want)
			}
		})
	}
}

// patchingReader patches the source once, the first time it is read, so the
// write lands between the plan Convert makes before its Tx and the one it
// makes inside.
type patchingReader struct {
	st      store.Store
	patched bool
}

func (r *patchingReader) Get(ctx context.Context, _, id string, face entity.Face) (*entity.Entity, bool, error) {
	e, err := r.st.GetEntity(ctx, entity.Ref{ID: id, Face: face})
	if err != nil {
		return nil, false, err
	}
	if !r.patched {
		r.patched = true
		changed := e.Clone()
		changed.SetString("body", "patched concurrently")
		if err := r.st.UpdateEntity(ctx, changed); err != nil {
			return nil, false, err
		}
	}
	return e, true, nil
}

func TestConvert_UsesTheFamilyReadInsideTheTx(t *testing.T) {
	t.Parallel()
	f := newConversionFixture(t, conversionOptions{
		visibility: func(st store.Store) entitymanager.CopyReader { return &patchingReader{st: st} },
	})
	f.seed(t, "NOTE-1", "note", map[string]any{"title": "Quick note", "body": "stale"})
	// A copied value changed after the read access check: the Tx neither
	// writes the stale value nor copies the unchecked one.
	_, err := f.conv.Convert(context.Background(),
		entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-1"})
	if !errors.Is(err, entitymanager.ErrConversionSourceChanged) {
		t.Fatalf("Convert = %v, want ErrConversionSourceChanged", err)
	}
	if _, err := f.st.GetEntity(context.Background(), entity.Ref{ID: "NOTE-1"}); err != nil {
		t.Fatalf("source should be untouched: %v", err)
	}
}

// failingReplaceStore fails the row replacement inside the Tx, after the
// rename. memstore cannot roll the rename back.
type failingReplaceStore struct{ store.Store }

func (s failingReplaceStore) Tx(ctx context.Context, fn func(store.Store) error) error {
	return s.Store.Tx(ctx, func(tx store.Store) error { return fn(failingReplaceTx{tx}) })
}

type failingReplaceTx struct{ store.Store }

var errReplace = errors.New("replace failed")

func (failingReplaceTx) ReplaceEntityFamily(context.Context, string, []store.FamilyReplacement) error {
	return errReplace
}

func TestConvert_RecordsARenameThatCouldNotRollBack(t *testing.T) {
	t.Parallel()
	f := newConversionFixture(t, conversionOptions{
		wrap: func(st store.Store) store.Store { return failingReplaceStore{st} },
	})
	f.seed(t, "NOTE-1", "note", map[string]any{"title": "Quick note"})
	_, err := f.conv.Convert(context.Background(),
		entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-1"})
	if !errors.Is(err, errReplace) {
		t.Fatalf("Convert = %v, want the replace error", err)
	}
	if !reflect.DeepEqual(f.hooks.renamed, [][2]string{{"NOTE-1", "TASK-001"}}) {
		t.Errorf("alias rewrites = %v", f.hooks.renamed)
	}
	if len(f.auditOps(audit.OpConvertEntity)) != 1 || len(f.hooks.versions) != 1 {
		t.Errorf("audit = %v, versions = %v", f.audit.Records(), f.hooks.versions)
	}
}

func TestConvert_HiddenRelationIsNeitherListedNorConfirmed(t *testing.T) {
	t.Parallel()
	f := newConversionFixture(t, conversionOptions{policy: conversionPolicy})
	ctx := f.asAlice(t, "")
	f.seed(t, "NOTE-1", "note", map[string]any{"title": "Quick note"})
	f.seed(t, "VAULT-1", "vault", nil)
	f.link(t, "NOTE-1", "stored-in", "VAULT-1")

	req := entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-1"}
	preview, err := f.conv.Preview(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.RelationsToDrop) != 0 || len(preview.InvalidRelations) != 0 {
		t.Fatalf("preview names a hidden relation: %+v", preview)
	}
	result, err := f.conv.Convert(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if result.RelationsDropped != 0 {
		t.Errorf("RelationsDropped = %d counts a hidden relation", result.RelationsDropped)
	}
	if f.hasRelation(result.Entity.ID, "stored-in", "VAULT-1") {
		t.Error("hidden relation was not dropped")
	}
	if len(f.auditOps(audit.OpDeleteRelation)) != 1 {
		t.Error("hidden drop was not audited")
	}
}

func TestConvert_HiddenRemapCapacityIsGeneric(t *testing.T) {
	t.Parallel()
	schema := strings.Replace(conversionSchema, "  followed-by:\n    label: Followed by\n",
		"  followed-by:\n    label: Followed by\n    max_incoming: 1\n", 1)
	f := newConversionFixture(t, conversionOptions{schema: schema, policy: conversionPolicy})
	f.seed(t, "NOTE-1", "note", map[string]any{"title": "Quick note"})
	f.seed(t, "VAULT-1", "vault", nil)
	f.seed(t, "TASK-500", "task", map[string]any{"title": "Holder"})
	f.link(t, "NOTE-1", "watched-by", "VAULT-1")
	f.link(t, "TASK-500", "followed-by", "VAULT-1")

	preview, err := f.conv.Preview(f.asAlice(t, ""),
		entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preview.InvalidRelations, []string{"a relation you cannot see cannot be converted"}) {
		t.Fatalf("invalid relations = %v", preview.InvalidRelations)
	}
}

func TestConvert_ClientCeilingBoundsTheDestination(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, principalType string
		allowed             bool
	}{
		{"no ceiling", "", true},
		{"ceiling allows creating the destination", "task-only", true},
		{"ceiling withholds creating the destination", "no-task-create", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newConversionFixture(t, conversionOptions{policy: conversionPolicy})
			f.seed(t, "NOTE-1", "note", map[string]any{"title": "Quick note"})
			ctx := f.asAlice(t, tc.principalType)
			offers := f.conv.Offers(ctx, "note", "NOTE-1")
			if len(offers) != 1 || offers[0].Allowed != tc.allowed {
				t.Errorf("offers = %+v, want allowed %v", offers, tc.allowed)
			}
			_, err := f.conv.Convert(ctx,
				entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-1"})
			if tc.allowed && err != nil {
				t.Fatalf("Convert: %v", err)
			}
			if !tc.allowed && !errors.Is(err, acl.ErrForbidden) {
				t.Fatalf("Convert = %v, want acl.ErrForbidden", err)
			}
		})
	}
}

// fieldHidingReader returns every row with field removed, as a `visible:`
// grant that omits it would.
type fieldHidingReader struct {
	st    store.Store
	field string
}

func (r fieldHidingReader) Get(ctx context.Context, _, id string, face entity.Face) (*entity.Entity, bool, error) {
	e, err := r.st.GetEntity(ctx, entity.Ref{ID: id, Face: face})
	if err != nil {
		return nil, false, err
	}
	seen := e.Clone()
	delete(seen.Properties, r.field)
	return seen, true, nil
}

func TestConvert_RefusesToCopyAnUnreadableField(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		body  any
		allow bool
	}{
		{"hidden value", "secret", false},
		{"no value", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newConversionFixture(t, conversionOptions{
				visibility: func(st store.Store) entitymanager.CopyReader { return fieldHidingReader{st: st, field: "body"} },
			})
			props := map[string]any{"title": "Quick note"}
			if tc.body != nil {
				props["body"] = tc.body
			}
			f.seed(t, "NOTE-1", "note", props)
			req := entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-1"}
			for name, call := range map[string]func() error{
				"preview": func() error { _, err := f.conv.Preview(context.Background(), req); return err },
				"convert": func() error { _, err := f.conv.Convert(context.Background(), req); return err },
			} {
				err := call()
				if tc.allow && err != nil {
					t.Errorf("%s: %v", name, err)
				}
				if !tc.allow && !errors.Is(err, entitymanager.ErrConversionInvalid) {
					t.Errorf("%s = %v, want ErrConversionInvalid", name, err)
				}
			}
		})
	}
}

const facedConversionSchema = `version: "1"
entities:
  note:
    label: Note
    id_prefix: "NOTE-"
    id_type: manual
    faces: {draft: {label: Draft}, final: {label: Final}}
    properties:
      title: {type: string, required: true}
      body: {type: string}
  task:
    label: Task
    id_prefix: "TASK-"
    id_type: manual
    faces: {active: {label: Active}, done: {label: Done}}
    properties:
      title: {type: string, required: true}
      summary: {type: string}
      editor_note: {type: string}
  peer:
    label: Peer
    id_prefix: "PEER-"
    id_type: manual
    properties: {title: {type: string}}
relations:
  relates-to:
    label: Relates to
    from: [note, task]
    to: [peer]
conversions:
  note-to-task:
    from: note
    to: task
    fields:
      title: "{{source.title}}"
      summary: "{{source.body}}"
    relations: {relates-to: relates-to}
    faces:
      draft: [active, done]
      final: [active]
`

func seedFaces(t *testing.T, st store.Store, faces ...entity.Face) {
	t.Helper()
	for _, face := range faces {
		e := entity.New("NOTE-1", "note")
		e.Face = face
		e.SetString("title", "Title "+string(face))
		e.SetString("body", "Body "+string(face))
		e.Content = "Content " + string(face)
		if err := st.CreateEntity(context.Background(), e); err != nil {
			t.Fatalf("seed %s: %v", face, err)
		}
	}
}

// recordingGuard is an active channel guard that records the ids it frees.
type recordingGuard struct {
	entitymanager.NopAnchorGuard
	renamedTo []string
}

func (*recordingGuard) Active() bool { return true }

func (g *recordingGuard) BeforeRename(
	ctx context.Context, newID string, free func(context.Context) error,
) (entitymanager.AnchorUndo, error) {
	g.renamedTo = append(g.renamedTo, newID)
	return nil, free(ctx)
}

// A target id that names the source must be refused before the channel
// guard runs: the guard frees an id for anything but the source, so it
// would detach the source's own channels.
func TestConvert_TargetIDNamingTheSourceSkipsTheGuard(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"NOTE-1", "note-1"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			guard := &recordingGuard{}
			f := newConversionFixture(t, conversionOptions{schema: facedConversionSchema, guard: guard})
			seedFaces(t, f.st, "final")
			_, err := f.conv.Convert(context.Background(), entitymanager.ConversionRequest{
				Definition: "note-to-task", SourceID: "NOTE-1", TargetID: target,
			})
			if !errors.Is(err, entitymanager.ErrConversionInvalid) {
				t.Fatalf("Convert = %v, want ErrConversionInvalid", err)
			}
			if len(guard.renamedTo) != 0 {
				t.Fatalf("guard ran for %v", guard.renamedTo)
			}
		})
	}
}

func TestConvert_MapsFaceFamilyAndRelations(t *testing.T) {
	t.Parallel()
	f := newConversionFixture(t, conversionOptions{schema: facedConversionSchema})
	ctx := context.Background()
	seedFaces(t, f.st, "draft", "final")
	f.seed(t, "PEER-1", "peer", nil)
	if _, err := f.st.CreateRelation(ctx, entity.RelationKey{From: "NOTE-1", FromFace: "draft", Type: "relates-to", To: "PEER-1"},
		&store.RelationData{}); err != nil {
		t.Fatal(err)
	}
	preview, err := f.conv.Preview(ctx, entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preview.RequiredFaces, []string{"draft"}) {
		t.Fatalf("required faces = %v", preview.RequiredFaces)
	}
	req := entitymanager.ConversionRequest{
		Definition: "note-to-task", SourceID: "NOTE-1", SourceFace: "draft", TargetID: "TASK-1",
		FaceMapping: map[string]string{"draft": "done"},
	}
	preview, err = f.conv.Preview(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	wantMoves := []entitymanager.ConversionMove{{SourceFace: "draft", TargetFace: "done"}, {SourceFace: "final", TargetFace: "active"}}
	if !reflect.DeepEqual(preview.Moves, wantMoves) {
		t.Errorf("moves = %+v, want %+v", preview.Moves, wantMoves)
	}
	result, err := f.conv.Convert(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Entity.Face != "done" {
		t.Fatalf("result face = %q", result.Entity.Face)
	}
	if _, err := f.st.GetRelation(ctx, entity.RelationKey{From: "TASK-1", FromFace: "done", Type: "relates-to", To: "PEER-1"}); err != nil {
		t.Fatalf("relation did not follow its face: %v", err)
	}
	prev := map[entity.Face]entity.Face{}
	for _, v := range f.hooks.versions {
		prev[v.Face] = v.PrevFace
	}
	if !reflect.DeepEqual(prev, map[entity.Face]entity.Face{"done": "draft", "active": "final"}) {
		t.Errorf("version face lineage = %v", prev)
	}
	slices.SortFunc(f.hooks.faceMoves, func(a, b faceMove) int { return strings.Compare(string(a.from), string(b.from)) })
	wantFaceMoves := []faceMove{{"task", "draft", "done"}, {"task", "final", "active"}}
	if !reflect.DeepEqual(f.hooks.faceMoves, wantFaceMoves) {
		t.Errorf("face moves = %+v, want %+v", f.hooks.faceMoves, wantFaceMoves)
	}
}

func TestConvert_ClientValuesApplyOnlyToTheSelectedFace(t *testing.T) {
	t.Parallel()
	f := newConversionFixture(t, conversionOptions{schema: facedConversionSchema})
	ctx := context.Background()
	seedFaces(t, f.st, "draft", "final")
	edited := "Edited draft content"
	req := entitymanager.ConversionRequest{
		Definition: "note-to-task", SourceID: "NOTE-1", SourceFace: "draft", TargetID: "TASK-1",
		FaceMapping: map[string]string{"draft": "done"},
		Properties:  map[string]any{"editor_note": "draft-only"}, Content: &edited,
	}
	if _, err := f.conv.Convert(ctx, req); err != nil {
		t.Fatal(err)
	}
	done, err := f.st.GetEntity(ctx, entity.Ref{ID: "TASK-1", Face: "done"})
	if err != nil {
		t.Fatal(err)
	}
	active, err := f.st.GetEntity(ctx, entity.Ref{ID: "TASK-1", Face: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if done.Content != edited || done.GetString("editor_note") != "draft-only" {
		t.Errorf("selected face = %+v", done)
	}
	if active.Content != "Content final" || active.GetString("editor_note") != "" || active.GetString("summary") != "Body final" {
		t.Errorf("other face = %+v", active)
	}
}

const facedConversionPolicy = `
roles:
  drafter:
    read: ["note@draft", task, peer]
    convert: [note]
  editor:
    read: [note, task, peer]
    convert: [note]
assignments:
  drafter: drafter
  alice: editor
`

func TestConvert_RefusesWhenAFaceIsUnreadable(t *testing.T) {
	t.Parallel()
	f := newConversionFixture(t, conversionOptions{schema: facedConversionSchema, policy: facedConversionPolicy})
	seedFaces(t, f.st, "draft", "final")
	ctx := entitymanager.BindAs(principal.With(context.Background(),
		principal.Principal{User: "drafter", Tool: principal.ToolDataEntry}), t, f.mgr)
	req := entitymanager.ConversionRequest{
		Definition: "note-to-task", SourceID: "NOTE-1", TargetID: "TASK-1",
		FaceMapping: map[string]string{"draft": "done"},
	}
	_, err := f.conv.Preview(ctx, req)
	if !errors.Is(err, entitymanager.ErrConversionInvalid) || strings.Contains(err.Error(), "final") {
		t.Fatalf("Preview = %v, want a refusal that names no face", err)
	}
	if _, err := f.conv.Convert(ctx, req); !errors.Is(err, entitymanager.ErrConversionInvalid) {
		t.Fatalf("Convert = %v, want ErrConversionInvalid", err)
	}
}

type faceMove struct {
	typ      string
	from, to entity.Face
}

// fakeLineageID is the lineage id conversionHooks reports for every edge.
const fakeLineageID = 42

type conversionHooks struct {
	renamed     [][2]string
	versions    []entitymanager.VersionRecord
	relVersions []entitymanager.RelationVersionRecord
	faceMoves   []faceMove
}

func (h *conversionHooks) EntityRenamed(_ context.Context, oldID, newID string) error {
	h.renamed = append(h.renamed, [2]string{oldID, newID})
	return nil
}
func (*conversionHooks) EntityDeleted(context.Context, string) error                  { return nil }
func (*conversionHooks) EntityFaceDeleted(context.Context, string, entity.Face) error { return nil }
func (h *conversionHooks) FaceMoved(_ context.Context, typ, _ string, from, to entity.Face) error {
	h.faceMoves = append(h.faceMoves, faceMove{typ, from, to})
	return nil
}
func (h *conversionHooks) RecordVersion(_ context.Context, v entitymanager.VersionRecord) error {
	h.versions = append(h.versions, v)
	return nil
}
func (h *conversionHooks) RecordRelationVersion(_ context.Context, v entitymanager.RelationVersionRecord) error {
	h.relVersions = append(h.relVersions, v)
	return nil
}
func (*conversionHooks) RelationRecordID(context.Context, entity.RelationKey) (int64, error) {
	return fakeLineageID, nil
}
