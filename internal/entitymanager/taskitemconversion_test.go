package entitymanager_test

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// taskItemSchema: a ticket's task items become tasks (has-subtask, the new
// task is the `to`) or blockers (blocks, the new task is the `from`). The
// policy type is faced, for the face rules.
const taskItemSchema = `version: "1"
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    id_type: manual
    properties:
      title: {type: string, required: true}
      project: {type: string}
      secret: {type: string}
  task:
    label: Task
    id_prefix: "TASK-"
    id_type: sequential
    properties:
      title: {type: string, required: true}
      project: {type: string}
      owner: {type: string, required: true}
      code: {type: string, unique: true}
      stage: {type: string}
  policy:
    label: Policy
    id_prefix: "POL-"
    id_type: sequential
    faces:
      draft: {label: Draft}
      published: {label: Published}
    properties:
      title: {type: string, required: true}
relations:
  has-subtask:
    label: Has subtask
    from: [ticket]
    to: [task, policy]
    max_outgoing: 2
  blocks:
    label: Blocks
    from: [task]
    to: [ticket]
  drafts:
    label: Drafts
    from: [policy]
    to: [ticket]
    scope: content
  reviews:
    label: Reviews
    from: [task]
    to: [ticket]
    max_incoming: 1
task_item_conversions:
  subtask:
    from: ticket
    to: task
    label: Convert to task
    relation: {type: has-subtask, as: to}
    fields:
      title: "{{item.text}}"
      project: "{{source.project}}"
      owner: "{{source.project}}-team"
  blocker:
    from: ticket
    to: task
    label: Convert to blocker
    item: remove
    relation: {type: blocks, as: from}
    fields:
      title: "Blocker: {{item.text}}"
      owner: "me"
  leak:
    from: ticket
    to: task
    label: Leak
    relation: {type: has-subtask, as: to}
    fields:
      title: "{{item.text}}"
      owner: "{{source.secret}}"
  coded:
    from: ticket
    to: task
    label: Coded
    relation: {type: has-subtask, as: to}
    fields:
      title: "{{item.text}}"
      owner: "o"
      code: "C-1"
  ask-owner:
    from: ticket
    to: task
    label: Ask owner
    relation: {type: has-subtask, as: to}
    fields:
      title: "{{item.text}}"
  draft-policy:
    from: ticket
    to: policy
    label: Draft a policy
    face: draft
    relation: {type: drafts, as: from}
    fields:
      title: "{{item.text}}"
`

type taskItemFixture struct {
	st    store.Store
	mgr   *entitymanager.Manager
	conv  *entitymanager.TaskItemConverter
	audit *audit.Memory
}

// bind opens the ACL request for the principal stamped on ctx.
func (f taskItemFixture) bind(ctx context.Context, t *testing.T) context.Context {
	t.Helper()
	return entitymanager.BindAs(ctx, t, f.mgr)
}

type taskItemOptions struct {
	st         store.Store
	policy     string
	fieldGate  entitymanager.FieldWriteGate
	visibility func(store.Store) entitymanager.CopyReader
	edges      entitymanager.TaskItemEdgeGate
	autos      []automation.Automation
}

// recordingEdges is a [entitymanager.TaskItemEdgeGate] that admits every
// edge and keeps the from entity it was asked about.
type recordingEdges struct{ from *entity.Entity }

func (g *recordingEdges) RelationCreatable(_ context.Context, from *entity.Entity, _ string) (bool, error) {
	g.from = from
	return true, nil
}

// TestTaskItemConvert_EdgeGateSeesTheRedactedSource pins that the relation
// gate judges the source as the caller sees it, as a hand-made relation
// create does, so a `when:` on a hidden field cannot answer on its value.
func TestTaskItemConvert_EdgeGateSeesTheRedactedSource(t *testing.T) {
	t.Parallel()
	edges := &recordingEdges{}
	f := newTaskItemFixture(t, taskItemOptions{
		edges:      edges,
		visibility: func(st store.Store) entitymanager.CopyReader { return fieldHidingReader{st: st, field: "secret"} },
	})
	_, err := f.conv.Convert(context.Background(), entitymanager.TaskItemRequest{
		Definition: "subtask", SourceID: "TKT-1", Text: "x",
	})
	require.NoError(t, err)
	require.NotNil(t, edges.from)
	if _, ok := edges.from.Properties["secret"]; ok {
		t.Fatal("the edge gate saw the hidden source field")
	}
	if edges.from.GetString("project") != "apollo" {
		t.Fatalf("the edge gate saw project = %q", edges.from.GetString("project"))
	}
}

// allowEdges is a [entitymanager.TaskItemEdgeGate] that admits every edge,
// or none when deny is set.
type allowEdges struct{ deny bool }

func (g allowEdges) RelationCreatable(context.Context, *entity.Entity, string) (bool, error) {
	return !g.deny, nil
}

func newTaskItemFixture(t *testing.T, opts taskItemOptions) taskItemFixture {
	t.Helper()
	meta, err := metamodel.Parse([]byte(taskItemSchema))
	require.NoError(t, err)
	transitions, err := statemachine.Compile(meta)
	require.NoError(t, err)
	st := opts.st
	if st == nil {
		st = memstore.New()
	}
	var gate acl.ACL = acl.NopACL{}
	if opts.policy != "" {
		policy, pErr := acl.LoadPolicyBytes([]byte(opts.policy))
		require.NoError(t, pErr)
		decl, dErr := acl.NewDeclarative(policy, aclstore.MustNewGraph(st), st)
		require.NoError(t, dErr)
		gate = decl
	}
	var visibility entitymanager.CopyReader = allowAllCopyVisibility(t, st)
	if opts.visibility != nil {
		visibility = opts.visibility(st)
	}
	var fields entitymanager.FieldWriteGate = entitymanager.AllowAllFieldGate{}
	if opts.fieldGate != nil {
		fields = opts.fieldGate
	}
	deps := entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.NewMemory(), ACL: gate,
		Transitions: transitions, FieldGate: fields,
		CopyReadGate: entitymanager.AllowAllCopyReadGate{}, CopyVisibility: visibility,
		AnchorGuard: entitymanager.NopAnchorGuard{},
	}
	auditor := deps.Audit.(*audit.Memory)
	if len(opts.autos) > 0 {
		engine := automation.NewEngine(opts.autos)
		runner, rErr := autocascade.New(autocascade.Deps{Engine: engine})
		require.NoError(t, rErr)
		deps.Automations, deps.Cascade = engine, runner
	}
	mgr, err := entitymanager.New(deps)
	require.NoError(t, err)
	if opts.fieldGate != nil {
		mgr = entitymanager.FieldGated(mgr)
	}
	var edges entitymanager.TaskItemEdgeGate = allowEdges{}
	if opts.edges != nil {
		edges = opts.edges
	}
	conv, err := entitymanager.NewTaskItemConverter(mgr, edges)
	require.NoError(t, err)
	f := taskItemFixture{st: st, mgr: mgr, conv: conv, audit: auditor}
	f.seed(t, "TKT-1", "", map[string]any{"title": "Parent", "project": "apollo", "secret": "s3cret"})
	return f
}

func (f taskItemFixture) seed(t *testing.T, id string, face entity.Face, props map[string]any) {
	t.Helper()
	e := entity.New(id, "ticket")
	e.Face = face
	maps.Copy(e.Properties, props)
	require.NoError(t, f.st.CreateEntity(context.Background(), e))
}

func (f taskItemFixture) taskCount(t *testing.T) int {
	t.Helper()
	n := 0
	for _, err := range f.st.ListEntities(context.Background(), store.EntityQuery{Type: "task", Faces: store.AllFaces()}) {
		require.NoError(t, err)
		n++
	}
	return n
}

func TestNewTaskItemConverter_RejectsNil(t *testing.T) {
	t.Parallel()
	if _, err := entitymanager.NewTaskItemConverter(nil, allowEdges{}); err == nil {
		t.Fatal("nil manager accepted")
	}
	mgr := newBareManager(t)
	if _, err := entitymanager.NewTaskItemConverter(mgr, nil); err == nil {
		t.Fatal("nil edge gate accepted")
	}
}

func newBareManager(t *testing.T) *entitymanager.Manager {
	t.Helper()
	meta, err := metamodel.Parse([]byte(taskItemSchema))
	require.NoError(t, err)
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: memstore.New(), Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: acl.NopACL{},
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	require.NoError(t, err)
	return mgr
}

func TestTaskItemConvert_CreatesEntityAndRelation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		definition string
		wantTitle  string
		wantOwner  string
		wantKey    func(id string) entity.RelationKey
	}{
		{"as to", "subtask", "Write the migration", "apollo-team",
			func(id string) entity.RelationKey {
				return entity.RelationKey{From: "TKT-1", Type: "has-subtask", To: id}
			}},
		{"as from", "blocker", "Blocker: Write the migration", "me",
			func(id string) entity.RelationKey { return entity.RelationKey{From: id, Type: "blocks", To: "TKT-1"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newTaskItemFixture(t, taskItemOptions{})
			ctx := f.bind(stampAlice(""), t)
			res, err := f.conv.Convert(ctx, entitymanager.TaskItemRequest{
				Definition: tc.definition, SourceID: "TKT-1", Text: "  Write the migration ",
			})
			require.NoError(t, err)
			got := res.Entity
			if got.Type != "task" || got.GetString("title") != tc.wantTitle || got.GetString("owner") != tc.wantOwner {
				t.Fatalf("created = %s %+v", got.Type, got.Properties)
			}
			if _, err := f.st.GetRelation(context.Background(), tc.wantKey(got.ID)); err != nil {
				t.Fatalf("relation missing: %v", err)
			}
			if n := len(auditRecords(f.audit, audit.OpCreateEntity)); n != 1 {
				t.Errorf("entity create records = %d, want 1", n)
			}
			if n := len(auditRecords(f.audit, audit.OpCreateRelation)); n != 1 {
				t.Errorf("relation create records = %d, want 1", n)
			}
		})
	}
}

func TestTaskItemConvert_TypedSourceValueAndMissingValue(t *testing.T) {
	t.Parallel()
	f := newTaskItemFixture(t, taskItemOptions{})
	f.seed(t, "TKT-2", "", map[string]any{"title": "No project"})
	res, err := f.conv.Convert(context.Background(), entitymanager.TaskItemRequest{
		Definition: "subtask", SourceID: "TKT-2", Text: "x",
	})
	require.NoError(t, err)
	if _, ok := res.Entity.Properties["project"]; ok {
		t.Errorf("an empty source value was copied: %+v", res.Entity.Properties)
	}
	if got := res.Entity.GetString("owner"); got != "-team" {
		t.Errorf("owner = %q, want %q", got, "-team")
	}
}

// The item text is the caller's. A placeholder inside it must stay text, in
// the commit and in the preview, or it would read source values.
func TestTaskItemConvert_ItemTextIsLiteral(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"{{source.secret}}", "{{new.secret}}", "{{today}}", "{{ item.text }}"} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			f := newTaskItemFixture(t, taskItemOptions{})
			req := entitymanager.TaskItemRequest{Definition: "subtask", SourceID: "TKT-1", Text: text}
			if _, err := f.conv.Preview(context.Background(), req); err != nil {
				t.Fatalf("preview: %v", err)
			}
			res, err := f.conv.Convert(context.Background(), req)
			require.NoError(t, err)
			if got := res.Entity.GetString("title"); got != text {
				t.Fatalf("title = %q, want %q", got, text)
			}
		})
	}
}

func TestTaskItemConvert_RefusesBadText(t *testing.T) {
	t.Parallel()
	f := newTaskItemFixture(t, taskItemOptions{})
	for name, text := range map[string]string{
		"empty":     "   ",
		"newline":   "a\nb",
		"tab":       "a\tb",
		"nul":       "a\x00b",
		"del":       "a\x7fb",
		"line sep":  "a b",
		"too long":  strings.Repeat("é", entitymanager.MaxTaskItemTextRunes+1),
		"bad utf-8": "a\xffb",
		"next line": "a\u0085b",
		"para sep":  "a b",
		"rtl ovr":   "a\u202eb",
		"isolate":   "a\u2066b",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := f.conv.Convert(context.Background(), entitymanager.TaskItemRequest{
				Definition: "subtask", SourceID: "TKT-1", Text: text,
			})
			if !errors.Is(err, entitymanager.ErrTaskItemInvalid) {
				t.Fatalf("Convert = %v, want ErrTaskItemInvalid", err)
			}
		})
	}
	// The zero-width joiner stays: emoji sequences need it.
	for _, ok := range []string{strings.Repeat("é", entitymanager.MaxTaskItemTextRunes), "\U0001F469\u200d\U0001F4BB fix"} {
		_, err := f.conv.Convert(context.Background(), entitymanager.TaskItemRequest{
			Definition: "subtask", SourceID: "TKT-1", Text: ok,
		})
		require.NoError(t, err)
	}
}

func TestTaskItemConvert_RefusesInvalidRequests(t *testing.T) {
	t.Parallel()
	f := newTaskItemFixture(t, taskItemOptions{})
	for _, tc := range []struct {
		name string
		req  entitymanager.TaskItemRequest
		want error
	}{
		{"unknown recipe", entitymanager.TaskItemRequest{Definition: "nope", SourceID: "TKT-1", Text: "x"},
			entitymanager.ErrUnknownTaskItemConversion},
		{"absent source", entitymanager.TaskItemRequest{Definition: "subtask", SourceID: "TKT-9", Text: "x"},
			entitymanager.ErrEntityNotFound},
		{"mapped field supplied", entitymanager.TaskItemRequest{Definition: "subtask", SourceID: "TKT-1", Text: "x",
			Properties: map[string]any{"title": "override"}}, entitymanager.ErrTaskItemInvalid},
		{"unknown field supplied", entitymanager.TaskItemRequest{Definition: "ask-owner", SourceID: "TKT-1", Text: "x",
			Properties: map[string]any{"nope": "v"}}, entitymanager.ErrTaskItemInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.conv.Convert(context.Background(), tc.req)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Convert = %v, want %v", err, tc.want)
			}
		})
	}
	if n := f.taskCount(t); n != 0 {
		t.Fatalf("refused requests created %d tasks", n)
	}
}

func TestTaskItemConvert_RequiredFieldsComeFromTheCaller(t *testing.T) {
	t.Parallel()
	f := newTaskItemFixture(t, taskItemOptions{})
	req := entitymanager.TaskItemRequest{Definition: "ask-owner", SourceID: "TKT-1", Text: "x"}
	preview, err := f.conv.Preview(context.Background(), req)
	require.NoError(t, err)
	if !reflect.DeepEqual(preview.Required, []string{"owner"}) {
		t.Fatalf("required = %v, want [owner]", preview.Required)
	}
	if _, err = f.conv.Convert(context.Background(), req); err == nil {
		t.Fatal("Convert without the required field succeeded")
	}
	req.Properties = map[string]any{"owner": "bob"}
	preview, err = f.conv.Preview(context.Background(), req)
	require.NoError(t, err)
	if len(preview.Required) != 0 {
		t.Fatalf("required after supplying = %v", preview.Required)
	}
	res, err := f.conv.Convert(context.Background(), req)
	require.NoError(t, err)
	if res.Entity.GetString("owner") != "bob" {
		t.Fatalf("owner = %q", res.Entity.GetString("owner"))
	}
}

func TestTaskItemConvert_RefusesToCopyAnUnreadableField(t *testing.T) {
	t.Parallel()
	f := newTaskItemFixture(t, taskItemOptions{
		visibility: func(st store.Store) entitymanager.CopyReader { return fieldHidingReader{st: st, field: "secret"} },
	})
	f.seed(t, "TKT-2", "", map[string]any{"title": "No secret"})
	// The answer is the same whether or not a hidden value is stored, so a
	// preflight cannot tell the two apart.
	answers := map[string]string{}
	for _, source := range []string{"TKT-1", "TKT-2"} {
		req := entitymanager.TaskItemRequest{Definition: "leak", SourceID: source, Text: "x"}
		for name, call := range map[string]func() error{
			"preview": func() error { _, err := f.conv.Preview(context.Background(), req); return err },
			"convert": func() error { _, err := f.conv.Convert(context.Background(), req); return err },
		} {
			err := call()
			if _, denied := errors.AsType[*acl.ForbiddenError](err); !denied {
				t.Fatalf("%s %s: %v, want forbidden", source, name, err)
			}
			if prev, seen := answers[name]; seen && prev != err.Error() {
				t.Errorf("%s answers %q for a stored secret and %q for none", name, prev, err.Error())
			}
			answers[name] = err.Error()
		}
	}
	// Only the converts are audited, each naming the source it probed.
	var probed []string
	for _, r := range auditRecords(f.audit, audit.OpDeniedWrite) {
		require.NotNil(t, r.Subject)
		probed = append(probed, r.Subject.Type+":"+r.Subject.ID)
	}
	sort.Strings(probed)
	if want := []string{"ticket:TKT-1", "ticket:TKT-2"}; !reflect.DeepEqual(probed, want) {
		t.Fatalf("denied-write subjects = %v, want %v", probed, want)
	}
	if n := f.taskCount(t); n != 0 {
		t.Fatalf("tasks = %d after refused conversions", n)
	}
	// A recipe that does not read the hidden field is unaffected.
	_, err := f.conv.Convert(context.Background(), entitymanager.TaskItemRequest{
		Definition: "subtask", SourceID: "TKT-1", Text: "x",
	})
	require.NoError(t, err)
}

// A field the item text fills is the caller's write, so the field gate
// judges it like a typed value.
func TestTaskItemConvert_GatesFieldsTheItemTextFills(t *testing.T) {
	t.Parallel()
	f := newTaskItemFixture(t, taskItemOptions{fieldGate: denyFieldGate{field: "title"}})
	_, err := f.conv.Convert(context.Background(), entitymanager.TaskItemRequest{
		Definition: "subtask", SourceID: "TKT-1", Text: "x",
	})
	if !errors.Is(err, errFieldDenied) {
		t.Fatalf("Convert = %v, want the field gate's denial", err)
	}
	// Mapped source values are the operator's: a gate on `project` does not
	// judge them.
	f = newTaskItemFixture(t, taskItemOptions{fieldGate: denyFieldGate{field: "project"}})
	_, err = f.conv.Convert(context.Background(), entitymanager.TaskItemRequest{
		Definition: "subtask", SourceID: "TKT-1", Text: "x",
	})
	require.NoError(t, err)
}

const taskItemPolicy = `
roles:
  editor:
    read: [ticket, task, policy]
    create: [ticket, task, policy]
    update: [ticket, task, policy]
  creator:
    read: [ticket, task]
    create: [task]
    update: [task]
  outsider:
    read: [task]
    create: [task]
    update: [task]
assignments:
  alice: editor
  bob: creator
  carol: outsider
`

func TestTaskItemConvert_Authorization(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		user       string
		definition string
		edges      entitymanager.TaskItemEdgeGate
		wantErr    func(error) bool
	}{
		{"editor", "alice", "subtask", nil, func(err error) bool { return err == nil }},
		// bob may create the task and the `blocks` edge from it, but may
		// not update the ticket whose body the item lives in.
		{"no update on source", "bob", "blocker", nil, isForbidden},
		{"no create on relation source", "bob", "subtask", nil, isForbidden},
		{"relation gate refuses", "alice", "subtask", allowEdges{deny: true}, isForbidden},
		{"source unreadable", "carol", "subtask", nil,
			func(err error) bool { return errors.Is(err, entitymanager.ErrEntityNotFound) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newTaskItemFixture(t, taskItemOptions{policy: taskItemPolicy, edges: tc.edges})
			ctx := f.bind(ctxWithPrincipal(tc.user, "data-entry"), t)
			_, err := f.conv.Convert(ctx, entitymanager.TaskItemRequest{
				Definition: tc.definition, SourceID: "TKT-1", Text: "x",
			})
			if !tc.wantErr(err) {
				t.Fatalf("Convert = %v", err)
			}
			if n := f.taskCount(t); (err == nil) != (n == 1) {
				t.Fatalf("tasks = %d after Convert = %v", n, err)
			}
			source, gErr := f.st.GetEntity(context.Background(), entity.Ref{ID: "TKT-1"})
			require.NoError(t, gErr)
			for _, o := range f.conv.Offers(ctx, source) {
				if o.Name == tc.definition && o.Allowed != (err == nil) {
					t.Errorf("offer allowed = %v, Convert = %v (reason %q)", o.Allowed, err, o.Reason)
				}
			}
			// Every refusal of a real write is audited, the edge gate's too,
			// as a hand-made relation POST audits it.
			_, wantAudit := errors.AsType[*acl.ForbiddenError](err)
			if got := len(auditRecords(f.audit, audit.OpDeniedWrite)) > 0; got != wantAudit {
				t.Errorf("denied-write recorded = %v, want %v (Convert = %v)", got, wantAudit, err)
			}
		})
	}
}

// A hidden source of the WRONG type answers exactly like an absent one, so
// the type check cannot confirm it exists.
func TestTaskItemConvert_HiddenSourceIsNotFoundWhateverItsType(t *testing.T) {
	t.Parallel()
	f := newTaskItemFixture(t, taskItemOptions{policy: taskItemPolicy})
	task := entity.New("TASK-1", "task")
	task.SetString("title", "t")
	task.SetString("owner", "o")
	require.NoError(t, f.st.CreateEntity(context.Background(), task))
	ctx := f.bind(ctxWithPrincipal("carol", "data-entry"), t)
	_, hidden := f.conv.Convert(ctx, entitymanager.TaskItemRequest{Definition: "subtask", SourceID: "TKT-1", Text: "x"})
	_, absent := f.conv.Convert(ctx, entitymanager.TaskItemRequest{Definition: "subtask", SourceID: "TKT-404", Text: "x"})
	if !errors.Is(hidden, entitymanager.ErrEntityNotFound) || !errors.Is(absent, entitymanager.ErrEntityNotFound) {
		t.Fatalf("hidden = %v, absent = %v", hidden, absent)
	}
	if strings.ReplaceAll(hidden.Error(), "TKT-1", "X") != strings.ReplaceAll(absent.Error(), "TKT-404", "X") {
		t.Fatalf("hidden and absent differ: %q vs %q", hidden, absent)
	}
}

func TestTaskItemConvert_FacedTargetAndContentScopedTail(t *testing.T) {
	t.Parallel()
	f := newTaskItemFixture(t, taskItemOptions{})
	res, err := f.conv.Convert(context.Background(), entitymanager.TaskItemRequest{
		Definition: "draft-policy", SourceID: "TKT-1", Text: "Retention",
	})
	require.NoError(t, err)
	if res.Entity.Face != "draft" {
		t.Fatalf("face = %q, want draft", res.Entity.Face)
	}
	key := entity.RelationKey{From: res.Entity.ID, FromFace: "draft", Type: "drafts", To: "TKT-1"}
	if _, err := f.st.GetRelation(context.Background(), key); err != nil {
		t.Fatalf("content-scoped relation not on the draft tail: %v", err)
	}
}

func TestTaskItemConvert_FacedSourceNeedsItsFace(t *testing.T) {
	t.Parallel()
	const schema = `version: "1"
entities:
  doc:
    label: Doc
    id_prefix: "DOC-"
    id_type: manual
    faces:
      draft: {label: Draft}
    properties: {title: {type: string}}
  task:
    label: Task
    id_prefix: "TASK-"
    id_type: sequential
    properties: {title: {type: string}}
relations:
  spawns:
    label: Spawns
    from: [doc]
    to: [task]
    scope: content
task_item_conversions:
  spawn:
    from: doc
    to: task
    label: Spawn
    relation: {type: spawns, as: to}
    fields: {title: "{{item.text}}"}
`
	meta, err := metamodel.Parse([]byte(schema))
	require.NoError(t, err)
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: acl.NopACL{},
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		CopyReadGate: entitymanager.AllowAllCopyReadGate{}, CopyVisibility: allowAllCopyVisibility(t, st),
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	require.NoError(t, err)
	conv, err := entitymanager.NewTaskItemConverter(mgr, allowEdges{})
	require.NoError(t, err)
	doc := entity.New("DOC-1", "doc")
	doc.Face = "draft"
	require.NoError(t, st.CreateEntity(context.Background(), doc))

	_, err = conv.Convert(context.Background(), entitymanager.TaskItemRequest{Definition: "spawn", SourceID: "DOC-1", Text: "x"})
	if !errors.Is(err, entitymanager.ErrTaskItemInvalid) {
		t.Fatalf("Convert without source face = %v, want ErrTaskItemInvalid", err)
	}
	res, err := conv.Convert(context.Background(), entitymanager.TaskItemRequest{
		Definition: "spawn", SourceID: "DOC-1", SourceFace: "draft", Text: "x",
	})
	require.NoError(t, err)
	key := entity.RelationKey{From: "DOC-1", FromFace: "draft", Type: "spawns", To: res.Entity.ID}
	if _, err := st.GetRelation(context.Background(), key); err != nil {
		t.Fatalf("relation not on the source face: %v", err)
	}
}

// A check that fails inside the Tx must fail before the entity write: the
// file and memory stores cannot roll it back.
func TestTaskItemConvert_FailedChecksLeaveNoEntity(t *testing.T) {
	t.Parallel()
	stores := map[string]func(t *testing.T) store.Store{
		"mem": func(*testing.T) store.Store { return memstore.New() },
		"fs":  openTaskItemFSStore,
	}
	for name, open := range stores {
		t.Run(name+"/capacity", func(t *testing.T) {
			t.Parallel()
			f := newTaskItemFixture(t, taskItemOptions{st: open(t)})
			for range 2 {
				_, err := f.conv.Convert(context.Background(), entitymanager.TaskItemRequest{
					Definition: "subtask", SourceID: "TKT-1", Text: "x",
				})
				require.NoError(t, err)
			}
			_, err := f.conv.Convert(context.Background(), entitymanager.TaskItemRequest{
				Definition: "subtask", SourceID: "TKT-1", Text: "x",
			})
			if err == nil {
				t.Fatal("third subtask exceeded max_outgoing")
			}
			if n := f.taskCount(t); n != 2 {
				t.Fatalf("tasks = %d after a refused conversion, want 2", n)
			}
		})
		t.Run(name+"/unique", func(t *testing.T) {
			t.Parallel()
			f := newTaskItemFixture(t, taskItemOptions{st: open(t)})
			existing := entity.New("TASK-50", "task")
			existing.SetString("title", "t")
			existing.SetString("owner", "o")
			existing.SetString("code", "C-1")
			require.NoError(t, f.st.CreateEntity(context.Background(), existing))
			_, err := f.conv.Convert(context.Background(), entitymanager.TaskItemRequest{
				Definition: "coded", SourceID: "TKT-1", Text: "x",
			})
			if !entitymanager.IsUniqueViolation(err) {
				t.Fatalf("Convert = %v, want a unique violation", err)
			}
			if n := f.taskCount(t); n != 1 {
				t.Fatalf("tasks = %d after a refused conversion, want 1", n)
			}
			if n := len(auditRecords(f.audit, audit.OpCreateRelation)); n != 0 {
				t.Fatalf("relation records = %d, want 0", n)
			}
		})
	}
}

func openTaskItemFSStore(t *testing.T) store.Store {
	t.Helper()
	fs := storage.NewMemFS()
	rooted, err := storage.NewRootedFS(fs, "/")
	require.NoError(t, err)
	st, err := fsstore.New(fsstore.Config{
		FS: fs, Rooted: rooted,
		EntitiesKey: "entities", RelationsKey: "relations",
		AttachmentsKey: "attachments", CacheKey: ".rela",
		Schemas: map[string]store.EntityTypeSchema{
			"ticket": {Plural: "tickets"},
			"task":   {Plural: "tasks"},
			"policy": {Plural: "policies"},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// A mapped source value that changes between the read check and the Tx
// refuses the conversion: the Tx would copy a value nobody checked.
func TestTaskItemConvert_SourceChangedBeforeTheTx(t *testing.T) {
	t.Parallel()
	f := newTaskItemFixture(t, taskItemOptions{
		visibility: func(st store.Store) entitymanager.CopyReader {
			return &patchingTicketReader{st: st}
		},
	})
	_, err := f.conv.Convert(context.Background(), entitymanager.TaskItemRequest{
		Definition: "subtask", SourceID: "TKT-1", Text: "x",
	})
	if !errors.Is(err, entitymanager.ErrTaskItemSourceChanged) {
		t.Fatalf("Convert = %v, want ErrTaskItemSourceChanged", err)
	}
	if n := f.taskCount(t); n != 0 {
		t.Fatalf("tasks = %d", n)
	}
}

// patchingTicketReader changes the ticket's project the first time the
// converter checks what the caller can read.
type patchingTicketReader struct {
	st      store.Store
	patched bool
}

func (r *patchingTicketReader) Get(ctx context.Context, _, id string, face entity.Face) (*entity.Entity, bool, error) {
	e, err := r.st.GetEntity(ctx, entity.Ref{ID: id, Face: face})
	if err != nil {
		return nil, false, err
	}
	if !r.patched {
		r.patched = true
		changed := e.Clone()
		changed.SetString("project", "gemini")
		if err := r.st.UpdateEntity(ctx, changed); err != nil {
			return nil, false, err
		}
	}
	return e, true, nil
}

func TestTaskItemConvert_RunsOnCreateAutomations(t *testing.T) {
	t.Parallel()
	f := newTaskItemFixture(t, taskItemOptions{autos: []automation.Automation{{
		Name: "stage-new-tasks",
		On:   automation.Trigger{Entity: []string{"task"}, Created: true},
		Do:   []automation.Action{{Set: "stage", Value: "triage"}},
	}}})
	res, err := f.conv.Convert(context.Background(), entitymanager.TaskItemRequest{
		Definition: "subtask", SourceID: "TKT-1", Text: "x",
	})
	require.NoError(t, err)
	stored, err := f.st.GetEntity(context.Background(), res.Entity.Ref())
	require.NoError(t, err)
	if stored.GetString("stage") != "triage" {
		t.Fatalf("on-create automation did not run: %+v", stored.Properties)
	}
}

func TestTaskItemOffers(t *testing.T) {
	t.Parallel()
	f := newTaskItemFixture(t, taskItemOptions{})
	source, err := f.st.GetEntity(context.Background(), entity.Ref{ID: "TKT-1"})
	require.NoError(t, err)
	offers := f.conv.Offers(context.Background(), source)
	var names []string
	for _, o := range offers {
		names = append(names, o.Name+":"+o.Item)
		if !o.Allowed {
			t.Errorf("%s refused: %s", o.Name, o.Reason)
		}
	}
	want := []string{"ask-owner:link", "blocker:remove", "coded:link", "draft-policy:link", "leak:link", "subtask:link"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("offers = %v, want %v", names, want)
	}
	task := entity.New("TASK-1", "task")
	if got := f.conv.Offers(context.Background(), task); got != nil {
		t.Fatalf("offers for a type with no recipe = %+v, want nil", got)
	}
}

// TestTaskItemOffers_JudgeTheSeenSource pins that an offer is judged as
// Convert judges it: a recipe copying a hidden field is not allowed, the
// edge gate sees the redacted row, and the probe records no denial.
func TestTaskItemOffers_JudgeTheSeenSource(t *testing.T) {
	t.Parallel()
	edges := &recordingEdges{}
	f := newTaskItemFixture(t, taskItemOptions{
		edges:      edges,
		visibility: func(st store.Store) entitymanager.CopyReader { return fieldHidingReader{st: st, field: "secret"} },
	})
	source, err := f.st.GetEntity(context.Background(), entity.Ref{ID: "TKT-1"})
	require.NoError(t, err)
	for _, o := range f.conv.Offers(context.Background(), source) {
		if o.Name == "leak" {
			if o.Allowed || !strings.Contains(o.Reason, "secret") {
				t.Errorf("leak copies a hidden field; offer = %+v", o)
			}
		} else if !o.Allowed {
			t.Errorf("%s refused: %s", o.Name, o.Reason)
		}
	}
	require.NotNil(t, edges.from)
	if _, ok := edges.from.Properties["secret"]; ok {
		t.Fatal("the edge gate saw the hidden source field")
	}
	if got := auditRecords(f.audit, audit.OpDeniedWrite); len(got) != 0 {
		t.Fatalf("an offer probe recorded %d denials", len(got))
	}
}

func isForbidden(err error) bool {
	_, ok := errors.AsType[*acl.ForbiddenError](err)
	return ok
}

func auditRecords(m *audit.Memory, op string) []audit.Record {
	var out []audit.Record
	for _, r := range m.Records() {
		if r.Op == op {
			out = append(out, r)
		}
	}
	return out
}
