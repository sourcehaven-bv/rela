package visibility_test

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// excludedFromChanges are the audit ops a change list never shows, each
// with why. Together with visibility.ChangeOps they must name every op.
var excludedFromChanges = map[string]string{
	audit.OpUpdateRecord:  "a record row, not entity content",
	audit.OpDeleteRecord:  "a record row, not entity content",
	audit.OpDeniedWrite:   "a refusal: nothing changed, and its summary names the denial",
	audit.OpACLBypass:     "operator elevation; the write it allowed has its own record",
	audit.OpACLBypassRead: "a read",
	audit.OpACLQuery:      "a read",
	audit.OpHistoryReveal: "a read",
	audit.OpPurgeVersion:  "operator action on history, global audit only",
	audit.OpTagVersion:    "a version label, not a content change",
	audit.OpUntagVersion:  "a version label, not a content change",
	audit.OpDataMigration: "operator maintenance",
	audit.OpConfigEdit:    "configuration, no entity",
	audit.OpDataGC:        "operator maintenance",
	audit.OpDataAdoptFace: "operator maintenance",
	audit.OpPerfSeed:      "operator maintenance",
	audit.OpFSImport:      "operator maintenance",
	audit.OpConfigImport:  "configuration, no entity",
	audit.OpTokenSet:      "credentials, no entity",
	audit.OpTokenDelete:   "credentials, no entity",
	audit.OpIDPRolesSet:   "identity provider state, no entity content",
	audit.OpAuditPrune:    "audit maintenance",
	audit.OpAuditImport:   "audit maintenance",
	audit.OpAuditRead:     "a read",
	audit.OpRemoteResolve: "remote sync bookkeeping",
	audit.OpRemoteDismiss: "remote sync bookkeeping",
	audit.OpRemoteLink:    "remote sync bookkeeping",
}

// TestChangeOpsClassifyEveryOp fails when the audit package gains an Op
// constant that is neither shown nor excluded, so a new op is a decision,
// not a default.
func TestChangeOpsClassifyEveryOp(t *testing.T) {
	ops := auditOpConstants(t)
	require.NotEmpty(t, ops)
	for _, op := range ops {
		shown := slices.Contains(visibility.ChangeOps, op)
		_, excluded := excludedFromChanges[op]
		require.NotEqualf(t, shown, excluded, "audit op %q must be in exactly one of ChangeOps and excludedFromChanges", op)
	}
}

// auditOpConstants parses the audit package for the values of its Op*
// string constants.
func auditOpConstants(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("../audit/*.go")
	require.NoError(t, err)
	var ops []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		require.NoError(t, err)
		ops = append(ops, opConstants(t, f)...)
	}
	return ops
}

// opConstants returns the values of the Op* string constants f declares.
func opConstants(t *testing.T, f *ast.File) []string {
	t.Helper()
	var ops []string
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, n := range vs.Names {
				if !strings.HasPrefix(n.Name, "Op") || i >= len(vs.Values) {
					continue
				}
				if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					v, err := strconv.Unquote(lit.Value)
					require.NoError(t, err)
					ops = append(ops, v)
				}
			}
		}
	}
	return ops
}

func TestChangeSummary(t *testing.T) {
	tests := []struct{ in, want string }{
		{"created", "created"},
		{"deleted (undoable)", "deleted (undoable)"},
		{"deleted face draft (cascade: 3 relations)", "deleted face draft"},
		{"updated: status,owner", "updated: status,owner"},
		{"renamed face published", "renamed face published"},
		{"converted from lead by qualify face draft", "converted from lead by qualify face draft"},
		{"purged after soft delete (2 relations)", "purged after soft delete"},
		{"renumbered order", "renumbered order"},
		{"created by conversion to deal", "created by conversion to deal"},
		{"rewrote mentions of SECRET-1 to SECRET-2", ""},
		{`copy "promote": SECRET-1@draft -> DOC-1@published (created=true)`, ""},
		{"updated: a, SECRET-1", ""},
		{`property="file" file="x.pdf" sha256=ab size=1 document="d" entry="SECRET-1"`, ""},
		{"", ""},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, visibility.ChangeSummary(tc.in))
		})
	}
}

func TestTriggerKind(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"automation", "automation"},
		{"automation:close-on-done", "automation:close-on-done"},
		{"schedule:nightly", "schedule:nightly"},
		{"renumber:order", "renumber:order"},
		{"cascade:delete-entity:SECRET-1", "cascade"},
		{"cascade:owner-delete:SECRET-1", "cascade"},
		{"rename:SECRET-1->SECRET-2", "rename"},
		{"restore-entity:SECRET-1", "restore-entity"},
		{"session:SECRET-1", "session"},
		{"automation:x:SECRET-1", "automation"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, visibility.TriggerKind(tc.in))
		})
	}
}

// fakeEntries serves ForEntity from a fixed newest-first list, applying the
// face, op and cursor filters, and counts its calls.
type fakeEntries struct {
	entries []audit.Entry
	calls   int
}

func (f *fakeEntries) ForEntity(_ context.Context, q audit.SubjectQuery, before int64, limit int) ([]audit.Entry, error) {
	f.calls++
	var out []audit.Entry
	for _, e := range f.entries {
		if before > 0 && e.Seq >= before {
			continue
		}
		if !slices.Contains(q.Ops, e.Record.Op) {
			continue
		}
		out = append(out, e)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeEntries) LifecycleStart(_ context.Context, q audit.SubjectQuery) (audit.Entry, bool, error) {
	for _, e := range f.entries {
		for _, s := range []*audit.Subject{e.Record.Subject, e.Record.After} {
			if audit.StartsLifecycle(e.Record, s) && s.ID == q.ID && s.Face == q.Face {
				return e, true, nil
			}
		}
	}
	return audit.Entry{}, false, nil
}

type readableSet map[string]string

func (r readableSet) ReadableTypes(_ context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	for _, id := range ids {
		if typ, ok := r[id]; ok {
			out[id] = typ
		}
	}
	return out, nil
}

func entityEntry(seq int64, op, typ string) audit.Entry {
	return audit.Entry{Seq: seq, Record: audit.Record{
		Op: op, Subject: &audit.Subject{Kind: "entity", Type: typ, ID: "DOC-1"},
		Principal: principal.Principal{User: "alice", Tool: principal.ToolDataEntry},
	}}
}

func relationEntry(seq int64, from, fromFace, to string) audit.Entry {
	return audit.Entry{Seq: seq, Record: audit.Record{
		Op: audit.OpCreateRelation,
		Subject: &audit.Subject{
			Kind: "relation", RelationType: "refs", FromID: from, FromFace: fromFace, ToID: to,
		},
		Summary: "created",
	}}
}

func changeSeqs(recs []visibility.ChangeRecord) []int64 {
	out := make([]int64, len(recs))
	for i, r := range recs {
		out[i] = r.Seq
	}
	return out
}

// TestEntityChanges_HidesRecordsNamingHiddenEntities: a relation record
// whose far end the reader cannot read is left out, a rename from this
// entity's old id is shown, an entity record of another type is left out,
// and a page reads past left-out records.
func TestEntityChanges_HidesRecordsNamingHiddenEntities(t *testing.T) {
	ctx := context.Background()
	src := &fakeEntries{entries: []audit.Entry{
		relationEntry(9, "DOC-1", "", "SECRET-1"),
		relationEntry(8, "DOC-1", "", "PUB-1"),
		relationEntry(7, "SECRET-2", "", "DOC-1"),
		relationEntry(6, "PUB-2", "draft", "DOC-1"),
		entityEntry(5, audit.OpUpdateEntity, "other"),
		{Seq: 4, Record: audit.Record{
			Op:     audit.OpRenameEntity,
			Before: &audit.Subject{Kind: "entity", Type: "doc", ID: "OLD-1"},
			After:  &audit.Subject{Kind: "entity", Type: "doc", ID: "DOC-1"},
		}},
		entityEntry(3, audit.OpUpdateEntity, "doc"),
		entityEntry(2, audit.OpDeniedWrite, "doc"),
		entityEntry(1, audit.OpCreateEntity, "doc"),
	}}
	readable := readableSet{"DOC-1": "doc", "PUB-1": "doc", "PUB-2": "doc"}
	q := visibility.ChangesQuery{Ref: entity.Ref{ID: "DOC-1"}, Type: "doc", Limit: 2}

	page, next, err := visibility.EntityChanges(ctx, src, readable, allFields{}, q)
	require.NoError(t, err)
	require.Equal(t, []int64{8, 4}, changeSeqs(page))
	require.Equal(t, int64(4), next)

	q.Before = next
	page, next, err = visibility.EntityChanges(ctx, src, readable, allFields{}, q)
	require.NoError(t, err)
	require.Equal(t, []int64{3, 1}, changeSeqs(page))
	require.Zero(t, next)
}

// TestEntityChanges_BodyNamesNoHiddenID: whatever the records say, a hidden
// id never reaches the served JSON.
func TestEntityChanges_BodyNamesNoHiddenID(t *testing.T) {
	ctx := context.Background()
	recs := []audit.Record{
		{Op: audit.OpUpdateEntity, Subject: &audit.Subject{Kind: "entity", Type: "doc", ID: "DOC-1"},
			TriggeredBy: "rename:SECRET-1->SECRET-2", Summary: "rewrote mentions of SECRET-1 to SECRET-2"},
		{Op: audit.OpCopyState, Subject: &audit.Subject{Kind: "entity", Type: "doc", ID: "DOC-1"},
			Summary: `copy "promote": SECRET-1@draft -> DOC-1@published (created=true)`},
		{Op: audit.OpDeleteRelation, Subject: &audit.Subject{
			Kind: "relation", RelationType: "refs", FromID: "DOC-1", ToID: "PUB-1"},
			TriggeredBy: "cascade:delete-entity:SECRET-1", Summary: "deleted"},
		{Op: audit.OpRestoreEntity, Subject: &audit.Subject{Kind: "entity", Type: "doc", ID: "DOC-1"},
			TriggeredBy: "restore-entity:SECRET-1", Summary: "restored"},
		{Op: audit.OpAttachRender, Subject: &audit.Subject{Kind: "entity", Type: "doc", ID: "DOC-1"},
			Summary: `property="f" file="a.pdf" sha256=x size=1 entry="SECRET-1"`},
		{Op: audit.OpDeleteEntity, Subject: &audit.Subject{Kind: "entity", Type: "doc", ID: "DOC-1"},
			Summary: "deleted (cascade: 7 relations)"},
	}
	src := &fakeEntries{}
	for i, r := range recs {
		src.entries = append([]audit.Entry{{Seq: int64(i + 1), Record: r}}, src.entries...)
	}
	readable := readableSet{"DOC-1": "doc", "PUB-1": "doc"}
	page, _, err := visibility.EntityChanges(ctx, src, readable, allFields{},
		visibility.ChangesQuery{Ref: entity.Ref{ID: "DOC-1"}, Type: "doc", Limit: 50})
	require.NoError(t, err)
	require.Len(t, page, len(recs))
	body, err := json.Marshal(page)
	require.NoError(t, err)
	require.NotContains(t, string(body), "SECRET")
	require.NotContains(t, string(body), "7 relations")
}

// TestEntityChanges_ScanCap: a run of hidden records longer than the scan
// cap serves a short page with a cursor, after a bounded number of reads.
func TestEntityChanges_ScanCap(t *testing.T) {
	ctx := context.Background()
	src := &fakeEntries{}
	for seq := int64(100); seq >= 2; seq-- {
		src.entries = append(src.entries, relationEntry(seq, "DOC-1", "", "SECRET-1"))
	}
	src.entries = append(src.entries, entityEntry(1, audit.OpCreateEntity, "doc"))
	q := visibility.ChangesQuery{Ref: entity.Ref{ID: "DOC-1"}, Type: "doc", Limit: 5}
	page, next, err := visibility.EntityChanges(ctx, src, readableSet{"DOC-1": "doc"}, allFields{}, q)
	require.NoError(t, err)
	require.Empty(t, page)
	require.NotZero(t, next)
	require.LessOrEqual(t, src.calls, 4)
}

// allFields shows every property.
type allFields struct{}

func (allFields) FieldVisibleOnEveryRow(context.Context, string, string) bool { return true }

// hiddenFields hides the named properties of every type.
type hiddenFields map[string]bool

func (h hiddenFields) FieldVisibleOnEveryRow(_ context.Context, _, property string) bool {
	return !h[property]
}

// TestEntityChanges_SummaryNamesNoHiddenProperty: an update summary names
// only the properties the reader sees on every row, and a relation update
// names none.
func TestEntityChanges_SummaryNamesNoHiddenProperty(t *testing.T) {
	ctx := context.Background()
	update := func(seq int64, summary string) audit.Entry {
		e := entityEntry(seq, audit.OpUpdateEntity, "doc")
		e.Record.Summary = summary
		return e
	}
	rel := relationEntry(0, "DOC-1", "", "PUB-1")
	rel.Record.Op, rel.Record.Summary = audit.OpUpdateRelation, "updated: note"
	src := &fakeEntries{entries: []audit.Entry{
		update(4, "updated: title,salary"),
		update(3, "updated: salary"),
		update(2, "updated"),
		update(1, "updated something unexpected"),
		rel,
	}}
	page, _, err := visibility.EntityChanges(ctx, src, readableSet{"DOC-1": "doc", "PUB-1": "doc"},
		hiddenFields{"salary": true},
		visibility.ChangesQuery{Ref: entity.Ref{ID: "DOC-1"}, Type: "doc", Limit: 10, Live: true})
	require.NoError(t, err)
	var got []string
	for _, c := range page {
		got = append(got, c.Summary)
	}
	require.Equal(t, []string{"updated: title", "updated", "updated", "updated", "updated"}, got)
}

// TestEntityChanges_StartFence: the change list of an id is served under a
// type only when the record that started its lifecycle started it as that
// type, or, with no such record, when the entity is live.
func TestEntityChanges_StartFence(t *testing.T) {
	ctx := context.Background()
	rel := relationEntry(3, "PUB-1", "", "GONE-1")
	created := func(typ string) audit.Entry {
		e := entityEntry(1, audit.OpCreateEntity, typ)
		e.Record.Subject.ID = "GONE-1"
		return e
	}
	readable := readableSet{"PUB-1": "doc"}
	for _, tc := range []struct {
		name    string
		entries []audit.Entry
		live    bool
		want    []int64
	}{
		{name: "started as another type", entries: []audit.Entry{rel, created("secret")}},
		{name: "started as the type", entries: []audit.Entry{rel, created("doc")}, want: []int64{3, 1}},
		{name: "no start, deleted", entries: []audit.Entry{rel}},
		{name: "no start, live", entries: []audit.Entry{rel}, live: true, want: []int64{3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := visibility.ChangesQuery{Ref: entity.Ref{ID: "GONE-1"}, Type: "doc", Limit: 10, Live: tc.live}
			page, _, err := visibility.EntityChanges(ctx, &fakeEntries{entries: tc.entries}, readable, allFields{}, q)
			require.NoError(t, err)
			require.Equal(t, tc.want, nilIfEmpty(changeSeqs(page)))
		})
	}
}

func nilIfEmpty(s []int64) []int64 {
	if len(s) == 0 {
		return nil
	}
	return s
}

// TestEntityChanges_OtherFaceLeftOut: a record of the id at another face, or
// a relation that leaves it from another face, is not on this face's list.
func TestEntityChanges_OtherFaceLeftOut(t *testing.T) {
	ctx := context.Background()
	atFace := func(seq int64, op, face string) audit.Entry {
		e := entityEntry(seq, op, "doc")
		e.Record.Subject.Face = face
		return e
	}
	src := &fakeEntries{entries: []audit.Entry{
		relationEntry(6, "DOC-1", "draft", "PUB-1"),
		relationEntry(5, "DOC-1", "final", "PUB-1"),
		atFace(4, audit.OpUpdateEntity, ""),
		atFace(3, audit.OpUpdateEntity, "draft"),
		atFace(2, audit.OpUpdateEntity, "final"),
		atFace(1, audit.OpCreateEntity, "final"),
	}}
	q := visibility.ChangesQuery{Ref: entity.Ref{ID: "DOC-1", Face: "final"}, Type: "doc", Limit: 10}
	page, _, err := visibility.EntityChanges(ctx, src, readableSet{"PUB-1": "doc"}, allFields{}, q)
	require.NoError(t, err)
	require.Equal(t, []int64{5, 2, 1}, changeSeqs(page))
}

// TestEntityChanges_FullPageStopsReading: a page filled from the first read
// returns its cursor without reading on, and a limit below 1 is refused.
func TestEntityChanges_FullPageStopsReading(t *testing.T) {
	ctx := context.Background()
	src := &fakeEntries{entries: []audit.Entry{
		entityEntry(3, audit.OpUpdateEntity, "doc"),
		relationEntry(2, "DOC-1", "", "SECRET-1"),
		entityEntry(1, audit.OpCreateEntity, "doc"),
	}}
	q := visibility.ChangesQuery{Ref: entity.Ref{ID: "DOC-1"}, Type: "doc", Limit: 1}
	page, next, err := visibility.EntityChanges(ctx, src, readableSet{}, allFields{}, q)
	require.NoError(t, err)
	require.Equal(t, []int64{3}, changeSeqs(page))
	require.Equal(t, int64(3), next)
	require.Equal(t, 1, src.calls)

	q.Limit = 0
	_, _, err = visibility.EntityChanges(ctx, src, readableSet{}, allFields{}, q)
	require.Error(t, err)
}
