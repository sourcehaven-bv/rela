package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// copyEdgesMeta declares a guarded same-entity promote on a faced type and a
// cross-entity copy on a faceless one. Both copy the content-scoped `cites`
// and `refs` edges. `cites` may point at a `secret`, which alice cannot read.
const copyEdgesMeta = `
entities:
  policy:
    label: Policy
    id_prefix: POL
    faces:
      draft: {}
      published: {}
    properties:
      title: { type: string }
  memo:
    label: Memo
    id_prefix: MEMO
    properties:
      title: { type: string }
  feature:
    label: Feature
    id_prefix: FEAT
    properties:
      title: { type: string }
  secret:
    label: Secret
    id_prefix: SEC
    properties:
      title: { type: string }
relations:
  cites:
    from: [policy, memo]
    to: [feature, secret]
    scope: content
  refs:
    from: [policy, memo]
    to: [feature]
    scope: content
copies:
  promote:
    from: policy@draft
    to: policy@published
    fields: all
    relations: { cites: merge, refs: merge }
    guard: { permission: promote }
  promote-replace:
    from: policy@draft
    to: policy@published
    fields: all
    relations: { cites: replace, refs: replace }
    guard: { permission: promote }
  memo-copy:
    from: memo
    to: new memo
    fields: { title: "{{new.title}}" }
    relations: { cites: merge, refs: merge }
  memo-replace:
    from: memo
    to: new memo
    fields: { title: "{{new.title}}" }
    relations: { cites: replace }
`

// copyEdgesPolicy configures alice's role for one copyEdgesApp.
type copyEdgesPolicy struct {
	readSecret bool                 // alice may read the secret type
	memoCreate bool                 // alice may create memos (and so their edges)
	memoDelete bool                 // alice may delete memos (and so their edges)
	relations  []acl.RelationGrant  // alice's relation grants on policy and memo
	sourceRels []entity.RelationKey // edges the copy source carries
	targetRels []entity.RelationKey // edges the target face carries before the copy
	extraPeers int                  // more readable features the source cites
	audit      *audit.Memory        // records the app's audit log when set
}

// copyEdgesApp builds an App over copyEdgesMeta whose store counts reads. It
// seeds POL-1 (draft and published), MEMO-1, MEMO-2, FEAT-1, FEAT-2 and SEC-1,
// and the edges p names.
func copyEdgesApp(t *testing.T, p copyEdgesPolicy) (*App, *acl.Declarative, *storetest.Counting) {
	t.Helper()
	meta, perr := metamodel.Parse([]byte(copyEdgesMeta))
	if perr != nil {
		t.Fatalf("parse metamodel: %v", perr)
	}
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	if err := fs.MkdirAll(paths.CacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	st := storetest.NewCounting(appbuildtest.New(meta, appbuildtest.WithFS(fs, paths)).Store())
	ctx := context.Background()
	seeds := []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: "draft", Properties: map[string]any{"title": "d"}},
		{ID: "POL-1", Type: "policy", Face: "published", Properties: map[string]any{"title": "p"}},
		{ID: "MEMO-1", Type: "memo", Properties: map[string]any{"title": "m1"}},
		{ID: "MEMO-2", Type: "memo", Properties: map[string]any{"title": "m2"}},
		{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "f1"}},
		{ID: "FEAT-2", Type: "feature", Properties: map[string]any{"title": "f2"}},
		{ID: "SEC-1", Type: "secret", Properties: map[string]any{"title": "s"}},
	}
	for i := range p.extraPeers {
		seeds = append(seeds, &entity.Entity{ID: fmt.Sprintf("FEAT-X%d", i), Type: "feature",
			Properties: map[string]any{"title": "x"}})
	}
	for _, e := range seeds {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for _, k := range slices.Concat(p.sourceRels, p.targetRels) {
		if _, err := st.CreateRelation(ctx, k, &store.RelationData{}); err != nil {
			t.Fatal(err)
		}
	}

	read := []string{"policy", "memo", "feature"}
	if p.readSecret {
		read = append(read, "secret")
	}
	create := []string{"feature"}
	if p.memoCreate {
		create = append(create, "memo")
	}
	var del []string
	if p.memoDelete {
		del = []string{"memo"}
	}
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"r": {
			Read: read, Create: create, Update: []string{"memo", "feature", "policy@draft"}, Delete: del,
			Permissions: []string{"promote"},
			Relations:   map[string][]acl.RelationGrant{"policy": p.relations, "memo": p.relations},
		}},
		Assignments: map[string]string{"alice": "r"},
	}, st)

	opts := []appbuildtest.Option{appbuildtest.WithFS(fs, paths), appbuildtest.WithStore(st),
		appbuildtest.WithDeclarative(d)}
	if p.audit != nil {
		opts = append(opts, appbuildtest.WithAudit(p.audit))
	}
	svc := appbuildtest.New(meta, opts...)
	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	app.acl = d
	app.schema.Publish(&Schema{Cfg: &Config{}, Meta: meta})
	app.setWorlds(fixedWorlds{scope: policyPublishedScope()})
	if err := SetWorldNeighbors(app, st, appbuild.RelationScopes(svc)); err != nil {
		t.Fatal(err)
	}
	resolver, err := affordances.New(meta, storeRelationLookup{st: st}, d)
	if err != nil {
		t.Fatal(err)
	}
	app.fieldResolver = &policyResolver{inner: resolver}
	st.Reset()
	return app, d, st
}

// copyAs invokes the named copy through the HTTP route, as alice.
func copyAs(t *testing.T, app *App, d *acl.Declarative, name, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/_copies/"+name,
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.copies.handleV1Copies(rec, asAlice(t, d, req))
	return rec
}

// edgesFrom lists the edges stored at from's face, as "type->to", sorted.
func edgesFrom(t *testing.T, st store.Store, from string, face entity.Face) []string {
	t.Helper()
	var out []string
	for rel, err := range st.ListRelations(t.Context(), store.RelationQuery{From: from, FromFace: &face}) {
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, rel.Type+"->"+rel.To)
	}
	slices.Sort(out)
	return out
}

func rel(from string, face entity.Face, typ, to string) entity.RelationKey {
	return entity.RelationKey{From: from, FromFace: face, Type: typ, To: to}
}

// copyEdgeSource is one copy form under test: the definition, its request,
// and where the source and target edges live.
type copyEdgeSource struct {
	name, def, body string
	from            string
	fromFace        entity.Face
	to              string
	toFace          entity.Face
}

var copyEdgeSources = []copyEdgeSource{
	{name: "faced promote", def: "promote", body: `{"source_id":"POL-1"}`,
		from: "POL-1", fromFace: "draft", to: "POL-1", toFace: "published"},
	{name: "cross-entity", def: "memo-copy", body: `{"source_id":"MEMO-1","target_id":"MEMO-9"}`,
		from: "MEMO-1", to: "MEMO-9"},
}

// TestCopy_SkipsEdgesTheCallerCouldNotCreate pins that a copy creates only
// the edges the caller could create by hand. An edge to a peer the caller
// cannot read, and an edge the relation gate refuses, is skipped; the copy
// itself succeeds. A hidden peer leaves no trace in the response: it is
// byte-identical to the response for a source that never had the edge.
func TestCopy_SkipsEdgesTheCallerCouldNotCreate(t *testing.T) {
	noRefs := false
	for _, src := range copyEdgeSources {
		allEdges := []entity.RelationKey{
			rel(src.from, src.fromFace, "cites", "FEAT-1"),
			rel(src.from, src.fromFace, "cites", "SEC-1"),
			rel(src.from, src.fromFace, "refs", "FEAT-2"),
		}
		withoutHidden := slices.DeleteFunc(slices.Clone(allEdges),
			func(k entity.RelationKey) bool { return k.To == "SEC-1" })
		for _, tc := range []struct {
			name string
			pol  copyEdgesPolicy
			twin *copyEdgesPolicy // a copy whose response must match byte for byte
			want []string
		}{
			{
				name: "hidden peer skipped",
				pol:  copyEdgesPolicy{memoCreate: true, sourceRels: allEdges},
				twin: &copyEdgesPolicy{memoCreate: true, sourceRels: withoutHidden},
				want: []string{"cites->FEAT-1", "refs->FEAT-2"},
			},
			{
				name: "affordance refusal skipped",
				pol: copyEdgesPolicy{memoCreate: true, readSecret: true, sourceRels: allEdges,
					relations: []acl.RelationGrant{{Relation: "refs", Create: &noRefs}}},
				want: []string{"cites->FEAT-1", "cites->SEC-1"},
			},
			{
				name: "all allowed",
				pol:  copyEdgesPolicy{memoCreate: true, readSecret: true, sourceRels: allEdges},
				want: []string{"cites->FEAT-1", "cites->SEC-1", "refs->FEAT-2"},
			},
		} {
			t.Run(src.name+"/"+tc.name, func(t *testing.T) {
				app, d, st := copyEdgesApp(t, tc.pol)
				rec := copyAs(t, app, d, src.def, src.body)
				if rec.Code != http.StatusOK {
					t.Fatalf("copy = %d %s", rec.Code, rec.Body)
				}
				if got := edgesFrom(t, st, src.to, src.toFace); !slices.Equal(got, tc.want) {
					t.Errorf("target edges = %v, want %v", got, tc.want)
				}
				if tc.twin != nil {
					twinApp, twinD, _ := copyEdgesApp(t, *tc.twin)
					twin := copyAs(t, twinApp, twinD, src.def, src.body)
					if twin.Code != rec.Code || twin.Body.String() != rec.Body.String() {
						t.Errorf("response with a hidden edge = %d %s\nwithout = %d %s",
							rec.Code, rec.Body, twin.Code, twin.Body)
					}
				}
			})
		}
	}
}

// TestCopy_ACLRefusedEdgesAreSkipped pins a cross-entity copy into an
// existing memo by a caller who may update memos but not create them, and so
// may not create their edges: the copy succeeds and writes no edge.
//
// The skipped edges leave no `denied-write` record: the copy never attempted
// them, so the audit log must not say it did.
func TestCopy_ACLRefusedEdgesAreSkipped(t *testing.T) {
	sink := audit.NewMemory()
	app, d, st := copyEdgesApp(t, copyEdgesPolicy{readSecret: true, audit: sink, sourceRels: []entity.RelationKey{
		rel("MEMO-1", "", "cites", "FEAT-1"),
		rel("MEMO-1", "", "refs", "FEAT-2"),
	}})
	rec := copyAs(t, app, d, "memo-copy", `{"source_id":"MEMO-1","target_id":"MEMO-2"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("copy = %d %s", rec.Code, rec.Body)
	}
	if got := edgesFrom(t, st, "MEMO-2", ""); len(got) != 0 {
		t.Errorf("target edges = %v, want none", got)
	}
	got, err := st.GetEntity(t.Context(), entity.Ref{ID: "MEMO-2"})
	if err != nil || got.Properties["title"] != "m1" {
		t.Errorf("the copy did not write the target: %v %v", got, err)
	}
	assertNoDeniedWrite(t, sink)
}

// assertNoDeniedWrite fails when sink holds a `denied-write` record.
func assertNoDeniedWrite(t *testing.T, sink *audit.Memory) {
	t.Helper()
	for _, r := range sink.Records() {
		if r.Op == audit.OpDeniedWrite {
			t.Errorf("denied-write recorded for a skipped edge: %+v", r)
		}
	}
}

// copyReplaceSources are the `replace` copy forms: a guarded face-to-face
// promote, which its guard authorizes, and a cross-entity copy, which the
// ACL authorizes per edge.
var copyReplaceSources = []copyEdgeSource{
	{name: "faced promote", def: "promote-replace", body: `{"source_id":"POL-1"}`,
		from: "POL-1", fromFace: "draft", to: "POL-1", toFace: "published"},
	{name: "cross-entity", def: "memo-replace", body: `{"source_id":"MEMO-1","target_id":"MEMO-2"}`,
		from: "MEMO-1", to: "MEMO-2"},
}

// TestCopyReplace_KeepsEdgesTheCallerCouldNotRemove pins that `replace`
// removes a target edge only when the caller could remove it by hand: the
// relation affordance gate must let the type be removed, and the ACL must
// allow the delete DeleteRelation asks. A kept edge is silent: the copy
// succeeds, the response is byte-identical to one for a target that never
// had the edge, and the audit log holds no `denied-write` record.
//
// A guarded face-to-face promote is exempt from the ACL check, as it is for
// creates (see copyEngine.authorizeCopy), so the ACL case runs cross-entity
// only.
func TestCopyReplace_KeepsEdgesTheCallerCouldNotRemove(t *testing.T) {
	noRemove := false
	for _, src := range copyReplaceSources {
		for _, tc := range []struct {
			name    string
			pol     copyEdgesPolicy
			want    []string
			aclOnly bool // a guarded face-to-face copy has no per-edge ACL check
		}{
			{
				name: "affordance refuses removal",
				pol: copyEdgesPolicy{memoCreate: true, memoDelete: true,
					relations: []acl.RelationGrant{{Relation: "cites", Remove: &noRemove}}},
				want: []string{"cites->FEAT-1", "cites->FEAT-2"},
			},
			{
				name:    "ACL refuses delete",
				pol:     copyEdgesPolicy{memoCreate: true},
				want:    []string{"cites->FEAT-1", "cites->FEAT-2"},
				aclOnly: true,
			},
			{
				name: "removal allowed",
				pol:  copyEdgesPolicy{memoCreate: true, memoDelete: true},
				want: []string{"cites->FEAT-1"},
			},
		} {
			if tc.aclOnly && src.toFace != "" {
				continue
			}
			t.Run(src.name+"/"+tc.name, func(t *testing.T) {
				pol := tc.pol
				pol.audit = audit.NewMemory()
				pol.sourceRels = []entity.RelationKey{rel(src.from, src.fromFace, "cites", "FEAT-1")}
				pol.targetRels = []entity.RelationKey{rel(src.to, src.toFace, "cites", "FEAT-2")}
				app, d, st := copyEdgesApp(t, pol)
				rec := copyAs(t, app, d, src.def, src.body)
				if rec.Code != http.StatusOK {
					t.Fatalf("copy = %d %s", rec.Code, rec.Body)
				}
				if got := edgesFrom(t, st, src.to, src.toFace); !slices.Equal(got, tc.want) {
					t.Errorf("target edges = %v, want %v", got, tc.want)
				}
				assertNoDeniedWrite(t, pol.audit)

				twinPol := tc.pol
				twinPol.sourceRels = pol.sourceRels
				twinApp, twinD, _ := copyEdgesApp(t, twinPol)
				twin := copyAs(t, twinApp, twinD, src.def, src.body)
				if twin.Code != rec.Code || twin.Body.String() != rec.Body.String() {
					t.Errorf("response with a target edge = %d %s\nwithout = %d %s",
						rec.Code, rec.Body, twin.Code, twin.Body)
				}
			})
		}
	}
}

// TestCopyReplace_KeepsEdgesToHiddenPeers pins that `replace` removes only
// the target edges the caller could remove by hand. An edge to a peer the
// caller cannot read survives, as it would a hand-made relation write.
func TestCopyReplace_KeepsEdgesToHiddenPeers(t *testing.T) {
	app, d, st := copyEdgesApp(t, copyEdgesPolicy{
		sourceRels: []entity.RelationKey{rel("POL-1", "draft", "cites", "FEAT-1")},
		targetRels: []entity.RelationKey{
			rel("POL-1", "published", "cites", "SEC-1"),
			rel("POL-1", "published", "cites", "FEAT-2"),
		},
	})
	rec := copyAs(t, app, d, "promote-replace", `{"source_id":"POL-1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("copy = %d %s", rec.Code, rec.Body)
	}
	want := []string{"cites->FEAT-1", "cites->SEC-1"}
	if got := edgesFrom(t, st, "POL-1", "published"); !slices.Equal(got, want) {
		t.Errorf("target edges = %v, want %v", got, want)
	}
}

// TestCopy_EdgeGateReadsDoNotGrowWithEdges pins the cost of gating a copy's
// edges: the store reads are the same at 10 and 50 copied edges.
func TestCopy_EdgeGateReadsDoNotGrowWithEdges(t *testing.T) {
	for _, src := range copyEdgeSources {
		t.Run(src.name, func(t *testing.T) {
			reads := func(n int) int {
				var rels []entity.RelationKey
				for i := range n {
					rels = append(rels, rel(src.from, src.fromFace, "cites", fmt.Sprintf("FEAT-X%d", i)))
				}
				rels = append(rels, rel(src.from, src.fromFace, "cites", "SEC-1"))
				app, d, st := copyEdgesApp(t, copyEdgesPolicy{memoCreate: true, extraPeers: n, sourceRels: rels})
				if rec := copyAs(t, app, d, src.def, src.body); rec.Code != http.StatusOK {
					t.Fatalf("copy = %d %s", rec.Code, rec.Body)
				}
				calls := st.Reads()
				if got := len(edgesFrom(t, st, src.to, src.toFace)); got != n {
					t.Fatalf("copied %d edges, want %d", got, n)
				}
				return calls
			}
			if r10, r50 := reads(10), reads(50); r10 != r50 {
				t.Errorf("copy reads grow with edges: %d at 10, %d at 50", r10, r50)
			}
		})
	}
	// `replace` adds the target edge read and the removal verdicts; those
	// must not grow with the edges either.
	for _, src := range copyReplaceSources {
		t.Run(src.name+"/replace", func(t *testing.T) {
			reads := func(n int) int {
				var source, target []entity.RelationKey
				for i := range n {
					source = append(source, rel(src.from, src.fromFace, "cites", fmt.Sprintf("FEAT-X%d", i)))
					target = append(target, rel(src.to, src.toFace, "cites", fmt.Sprintf("FEAT-X%d", n+i)))
				}
				target = append(target, rel(src.to, src.toFace, "cites", "SEC-1"))
				app, d, st := copyEdgesApp(t, copyEdgesPolicy{memoCreate: true, memoDelete: true,
					extraPeers: 2 * n, sourceRels: source, targetRels: target})
				if rec := copyAs(t, app, d, src.def, src.body); rec.Code != http.StatusOK {
					t.Fatalf("copy = %d %s", rec.Code, rec.Body)
				}
				calls := st.Reads()
				// The n source edges, plus the hidden peer's edge, which stays.
				if got := len(edgesFrom(t, st, src.to, src.toFace)); got != n+1 {
					t.Fatalf("target has %d edges, want %d", got, n+1)
				}
				return calls
			}
			if r10, r50 := reads(10), reads(50); r10 != r50 {
				t.Errorf("replace reads grow with edges: %d at 10, %d at 50", r10, r50)
			}
		})
	}
}
