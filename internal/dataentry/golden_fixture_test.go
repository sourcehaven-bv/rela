package dataentry_test

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/comments/memcomments"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/dataentrywire"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/piles/kvpiles"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// The golden corpus app (golden_parity_test.go) is built the way rela-server's
// buildApp builds one: NewApp over the services, then dataentrywire.Services,
// the security config and the identity wiring. buildApp itself lives in
// package main and cannot be imported, so newGoldenApp repeats its calls in
// its order. The services come from appbuildtest rather than appbuild.New:
// appbuild.New opens its own store and version history, and the corpus needs
// both counted. Only those two are test doubles.

// goldenFixtureDir holds the operator files of the corpus project. Every app
// gets a fresh copy, so a change to one of them (an acl.yaml grant, say)
// shows up as a golden diff.
const goldenFixtureDir = "testdata/golden/fixture"

// goldenHost is the bind address the corpus app is told it serves, and the
// Host and Origin every corpus request carries.
const goldenHost = "127.0.0.1:8080"

// Identity headers. goldenUserHeader carries a trusted login in header
// mode; goldenAssertionHeader carries a signed assertion when the JWT gate
// is wired, which goldenVerifier checks.
const (
	goldenUserHeader      = "X-Golden-User"
	goldenAssertionHeader = "X-Golden-Assertion"
)

// goldenEpoch is the fixed time of every timestamp the fixture writes itself.
var goldenEpoch = time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC)

// goldenACLMode names the ACL implementation a corpus app is built with.
// They are the arms of the type switches on the ACL implementation.
type goldenACLMode string

const (
	goldenDeclarative goldenACLMode = "declarative"
	goldenNop         goldenACLMode = "nop"
	goldenReadOnly    goldenACLMode = "readonly"
)

// goldenVariant is one way of wiring the corpus app.
type goldenVariant struct {
	name string
	mode goldenACLMode
	// jwt wires the JWT identity gate instead of the trusted header, which
	// is what makes `unmatched_principal: reject` take effect.
	jwt bool
}

// goldenApp is a corpus app with its router and its read recorders.
type goldenApp struct {
	app      *dataentry.App
	router   http.Handler
	calls    *storetest.Counting
	breadth  *storetest.Breadth
	bodies   *storetest.BodyWatch
	versions *goldenVersions
	// builtFrom is when the build began. The store stamps the fixture rows
	// as it seeds them, so timestamps minted from here on are the run's own.
	builtFrom time.Time
}

// reset clears every recorder, so the next request is measured alone.
func (g *goldenApp) reset() {
	g.calls.Reset()
	g.breadth.Reset()
	g.bodies.Reset()
	g.versions.reset()
}

// newGoldenApp builds a fresh corpus app for v.
func newGoldenApp(t *testing.T, v goldenVariant) *goldenApp {
	t.Helper()
	builtFrom := time.Now()
	ctx := context.Background()

	fsys, paths := goldenFilesystem(t)
	seedGoldenPiles(ctx, t, paths)
	schemaYAML, err := fs.ReadFile(os.DirFS(goldenFixtureDir), "schema.yaml")
	if err != nil {
		t.Fatalf("read schema.yaml: %v", err)
	}
	meta, err := metamodel.Parse(schemaYAML)
	if err != nil {
		t.Fatalf("parse schema.yaml: %v", err)
	}

	base := memstore.New()
	seedGoldenStore(ctx, t, base)
	bodies := storetest.NewBodyWatch(base)
	breadth := storetest.NewBreadth(bodies)
	calls := storetest.NewCounting(breadth)

	commentSvc, err := comments.NewService(seedGoldenComments(ctx, t), func() time.Time { return goldenEpoch })
	if err != nil {
		t.Fatalf("comments service: %v", err)
	}
	opts := []appbuildtest.Option{
		appbuildtest.WithFS(fsys, paths),
		appbuildtest.WithStore(calls),
		appbuildtest.WithComments(commentSvc),
	}
	switch v.mode {
	case goldenDeclarative:
		opts = append(opts, appbuildtest.WithDeclarative(goldenDeclarativeACL(t, meta, calls)))
	case goldenNop:
		opts = append(opts, appbuildtest.WithACL(acl.NopACL{}))
	case goldenReadOnly:
		opts = append(opts, appbuildtest.WithACL(acl.ReadOnlyACL{}))
	default:
		t.Fatalf("unknown ACL mode %q", v.mode)
	}
	svc := appbuildtest.New(meta, opts...)
	t.Cleanup(func() { _ = svc.Close() })

	// As cmd/rela-server's buildApp: the resolver from the services and the
	// command authorizer chosen from the ACL. The corpus binds to loopback
	// and does not pass --allow-unauthenticated-commands.
	resolver, err := dataentry.ResolverFromServices(svc)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	authz, err := dataentry.SelectCommandAuthorizer(
		svc.ACL(), svc.ACLDeclarative(), true, false, dataentry.CommandAuthNotifier{})
	if err != nil {
		t.Fatalf("command authorizer: %v", err)
	}
	versions := newGoldenVersions()
	app, err := dataentry.NewApp(
		svc.FS(), svc.Paths(), svc.ProjectFiles(), svc.Templater(), svc.Meta(), svc.Store(), versions,
		svc.EntityManager(), svc.Searcher(), svc.VisibleSearcher(), svc.ACL(),
		resolver, svc.Audit(), svc.State(), authz, appbuild.CompiledWorlds(svc),
	)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if err := dataentrywire.Services(app, svc); err != nil {
		t.Fatalf("wire services: %v", err)
	}
	if err := app.SetSecurityConfig(dataentry.SecurityConfig{BindAddress: goldenHost}); err != nil {
		t.Fatalf("security config: %v", err)
	}
	wireGoldenIdentity(t, app, v.jwt)
	// rela-server mounts the remote MCP endpoint after the identity wiring,
	// and only over the JWT gate (-mcp requires it).
	if v.jwt {
		if err := dataentrywire.RemoteMCP(app, svc, "golden"); err != nil {
			t.Fatalf("remote MCP: %v", err)
		}
	}

	g := &goldenApp{
		app: app, router: app.NewRouter(),
		calls: calls, breadth: breadth, bodies: bodies, versions: versions,
		builtFrom: builtFrom,
	}
	g.reset()
	return g
}

// wireGoldenIdentity installs the identity source the way
// wirePrincipalResolvers in cmd/rela-server does, minus the environment
// resolver: a developer's RELA_DATAENTRY_USER must not change the corpus.
func wireGoldenIdentity(t *testing.T, app *dataentry.App, jwt bool) {
	t.Helper()
	if jwt {
		if err := app.SetJWTGate(dataentry.JWTGateConfig{Verifier: goldenVerifier{}, HeaderName: goldenAssertionHeader}); err != nil {
			t.Fatalf("jwt gate: %v", err)
		}
		app.SetPrincipalHeader(goldenAssertionHeader)
		return
	}
	app.SetPrincipalResolver(dataentry.HeaderPrincipalResolver(goldenUserHeader))
	app.SetPrincipalHeader(goldenUserHeader)
}

// goldenVerifier accepts an assertion of the form "signed:<subject>" and
// refuses anything else, standing in for the signature check. It asserts the
// roles in goldenAssertedRoles.
type goldenVerifier struct{}

// goldenAssertedRoles are the roles the identity provider asserts per subject.
var goldenAssertedRoles = map[string][]string{goldenUnmatched: {"staff"}}

func (goldenVerifier) VerifyAssertion(_ context.Context, raw string) (dataentry.AssertedIdentity, error) {
	subject, ok := strings.CutPrefix(raw, "signed:")
	if !ok || subject == "" {
		return dataentry.AssertedIdentity{}, errors.New("golden: assertion is not signed")
	}
	return dataentry.AssertedIdentity{Subject: subject, Email: subject, Roles: goldenAssertedRoles[subject]}, nil
}

// goldenDeclarativeACL loads the fixture acl.yaml the way appbuild does:
// parse, validate against the schema, and build over the counted store.
func goldenDeclarativeACL(t *testing.T, meta *metamodel.Metamodel, st store.Store) *acl.Declarative {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(goldenFixtureDir, "acl.yaml"))
	if err != nil {
		t.Fatalf("read acl.yaml: %v", err)
	}
	policy, err := acl.ParsePolicy(data, "acl.yaml")
	if err != nil {
		t.Fatalf("parse acl.yaml: %v", err)
	}
	if vErr := appbuild.ValidateACLPolicy(policy, meta); vErr != nil {
		t.Fatalf("validate acl.yaml: %v", vErr)
	}
	d, err := acl.NewDeclarative(policy, acl.NewStoreGraph(st), st,
		acl.WithPrincipalLookup(acl.NewStorePrincipalLookup(st)))
	if err != nil {
		t.Fatalf("build acl: %v", err)
	}
	return d
}

// goldenFilesystem copies the fixture files into a fresh project directory.
// The project lives on disk, as on the filesystem tier: rela-server reads its
// operator files from the project root, and a command runs in it.
func goldenFilesystem(t *testing.T) (storage.FS, *project.Context) {
	t.Helper()
	root := t.TempDir()
	// The entity files are not the store (the corpus runs on memory); the
	// one there is a conflicted copy of TKT-4 for the conflict routes.
	paths := &project.Context{
		Root: root, CacheDir: filepath.Join(root, ".rela"),
		EntitiesDir: filepath.Join(root, "entities"), RelationsDir: filepath.Join(root, "relations"),
	}
	if err := os.MkdirAll(paths.CacheDir, 0o755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	if err := os.CopyFS(root, os.DirFS(goldenFixtureDir)); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return storage.NewSafeFS(storage.NewOsFS()), paths
}

// The fixture's people. Each logs in with its email; outsider is a known
// person with no grants, and goldenUnmatched has no person at all.
const (
	goldenAdmin     = "admin@example.test"
	goldenEditor    = "editor@example.test"
	goldenViewer    = "viewer@example.test"
	goldenOutsider  = "outsider@example.test"
	goldenUnmatched = "stranger@example.test"
)

// seedGoldenStore writes the fixture graph with fixed ids.
//
// The viewer watches TKT-1..3 and so reads exactly those tickets; TKT-4..6
// are hidden rows for it. TKT-1 is open and TKT-2 closed, so the `secret`
// grant's `when:` takes both arms; TKT-1 contains a closed ticket and TKT-3
// contains none, so the related() grant on `note` takes both arms too. TKT-1
// contains the hidden TKT-4, so the gantt has a hidden child to roll up, and
// TKT-3 implements POL-2, a policy with only a draft, which neither editor
// nor viewer may read. The bodies of TKT-2 and the hidden TKT-5 mention TKT-1.
// TKT-7 is soft-deleted.
func seedGoldenStore(ctx context.Context, t *testing.T, st store.Store) {
	t.Helper()
	people := []struct{ id, title, email string }{
		{"PERS-admin", "Ada Admin", goldenAdmin},
		{"PERS-editor", "Eddie Editor", goldenEditor},
		{"PERS-viewer", "Vera Viewer", goldenViewer},
		{"PERS-outsider", "Otto Outsider", goldenOutsider},
	}
	for _, p := range people {
		mustSeedEntity(ctx, t, st, &entity.Entity{
			ID: p.id, Type: "person", Properties: map[string]any{"title": p.title, "email": p.email},
		})
	}
	tickets := []struct {
		id, title, status, secret, note, start, due, body string
	}{
		{"TKT-1", "Alpha", "open", "alpha-secret", "alpha-note", "2025-01-06", "2025-01-17", "Alpha body."},
		{"TKT-2", "Bravo", "closed", "bravo-secret", "bravo-note", "2025-01-08", "2025-01-10", "Follows `TKT-1`."},
		{"TKT-3", "Charlie", "open", "charlie-secret", "charlie-note", "2025-02-03", "2025-02-14", ""},
		{"TKT-4", "Delta", "open", "delta-secret", "delta-note", "2025-01-20", "2025-03-28", "Hidden child."},
		{"TKT-5", "Echo", "closed", "echo-secret", "", "", "2025-02-20", "Also follows `TKT-1`."},
		{"TKT-6", "Foxtrot", "open", "foxtrot-secret", "", "", "", ""},
	}
	for _, tk := range tickets {
		props := map[string]any{
			"title": tk.title, "status": tk.status, "secret": tk.secret, "budget": strings.ToLower(tk.title) + "-budget",
		}
		for k, v := range map[string]string{"note": tk.note, "start": tk.start, "due": tk.due} {
			if v != "" {
				props[k] = v
			}
		}
		if tk.status == "closed" {
			props["closed_on"] = tk.due + "T17:00:00Z"
		}
		if tk.id == "TKT-1" {
			props["screenshot"] = "attachments/TKT-1/screenshot/shot.txt"
		}
		mustSeedEntity(ctx, t, st, &entity.Entity{ID: tk.id, Type: "ticket", Properties: props, Content: tk.body})
	}
	if err := st.AttachFamilyFile(ctx, "TKT-1", "screenshot", "shot.txt", strings.NewReader("shot bytes")); err != nil {
		t.Fatalf("attach: %v", err)
	}
	// TKT-7 is deleted and may be restored.
	mustSeedEntity(ctx, t, st, &entity.Entity{
		ID: "TKT-7", Type: "ticket", Properties: map[string]any{"title": "Hotel", "status": "open"},
	})
	sd, ok := st.(store.SoftDeleteProvider)
	if !ok {
		t.Fatal("the corpus store keeps no soft delete")
	}
	if _, err := sd.SoftDelete().MarkDeleted(ctx, "TKT-7", goldenAdmin); err != nil {
		t.Fatalf("delete TKT-7: %v", err)
	}
	for _, p := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: "draft", Properties: map[string]any{"title": "Access (draft)", "stage": "to-do"}},
		{ID: "POL-1", Type: "policy", Face: "published", Properties: map[string]any{"title": "Access", "stage": "done"}},
		{ID: "POL-2", Type: "policy", Face: "draft", Properties: map[string]any{"title": "Unreleased", "stage": "to-do"}},
	} {
		mustSeedEntity(ctx, t, st, p)
	}
	for _, k := range []entity.RelationKey{
		{From: "PERS-viewer", Type: "watches", To: "TKT-1"},
		{From: "PERS-viewer", Type: "watches", To: "TKT-2"},
		{From: "PERS-viewer", Type: "watches", To: "TKT-3"},
		{From: "PERS-editor", Type: "watches", To: "TKT-4"},
		{From: "TKT-1", Type: "contains", To: "TKT-2"},
		{From: "TKT-1", Type: "contains", To: "TKT-4"},
		{From: "TKT-1", Type: "implements", To: "POL-1"},
		{From: "TKT-3", Type: "implements", To: "POL-2"},
		{From: "TKT-5", Type: "implements", To: "POL-1"},
	} {
		if _, err := st.CreateRelation(ctx, k, nil); err != nil {
			t.Fatalf("seed relation %s: %v", k, err)
		}
	}
}

func mustSeedEntity(ctx context.Context, t *testing.T, st store.Store, e *entity.Entity) {
	t.Helper()
	if err := st.CreateEntity(ctx, e); err != nil {
		t.Fatalf("seed %s@%s: %v", e.ID, e.Face, err)
	}
}

// seedGoldenComments returns a comment store holding one comment on a
// visible ticket and one on a hidden one, with fixed ids and times.
func seedGoldenComments(ctx context.Context, t *testing.T) comments.Store {
	t.Helper()
	st := memcomments.New()
	for _, c := range []struct {
		target comments.Target
		id     string
	}{
		{comments.Target{Type: "ticket", ID: "TKT-1"}, "c-visible"},
		{comments.Target{Type: "ticket", ID: "TKT-4"}, "c-hidden"},
	} {
		if err := st.Add(ctx, c.target, comments.Comment{
			ID: c.id, Author: goldenAdmin, CreatedAt: goldenEpoch,
			Anchor: comments.Anchor{Kind: comments.AnchorProperty, Ref: "title"},
			Body:   "Comment " + c.id,
		}); err != nil {
			t.Fatalf("seed comment %s: %v", c.id, err)
		}
	}
	return st
}

// seedGoldenPiles writes one pile per fixture person into the project's
// cache directory, where the services' piles backend reads them: each holds
// a visible and a hidden ticket, with fixed ids and times. The owners are
// person ids and logins both, so the recording shows which form a lookup
// uses.
func seedGoldenPiles(ctx context.Context, t *testing.T, paths *project.Context) {
	t.Helper()
	rfs, err := storage.NewRootedFS(storage.NewSafeFS(storage.NewOsFS()), paths.CacheDir)
	if err != nil {
		t.Fatalf("piles fs: %v", err)
	}
	locker, err := filelock.New(filepath.Join(paths.CacheDir, "piles.lock"))
	if err != nil {
		t.Fatalf("piles lock: %v", err)
	}
	st, err := kvpiles.New(state.NewFSKV(rfs), locker)
	if err != nil {
		t.Fatalf("piles store: %v", err)
	}
	refs := []entity.Ref{{ID: "TKT-1"}, {ID: "TKT-5"}}
	for i, owner := range []string{
		"PERS-admin", "PERS-editor", "PERS-viewer", "PERS-outsider",
		goldenAdmin, goldenEditor, goldenViewer, goldenOutsider, goldenUnmatched,
	} {
		p := piles.Pile{
			ID: "PIL-GOLD" + string(rune('A'+i)), Owner: owner, Name: "Reading",
			Created: goldenEpoch, Updated: goldenEpoch,
		}
		if cErr := st.CreatePile(ctx, p, refs, 10, 10); cErr != nil {
			t.Fatalf("seed pile for %s: %v", owner, cErr)
		}
	}
}

// goldenVersions is the corpus version history: two versions of TKT-1, one
// of TKT-3, one of the hidden TKT-4, and two versions of the TKT-1 implements
// POL-1 edge. It counts its reads, which the store recorders cannot see.
type goldenVersions struct {
	mu    sync.Mutex
	calls map[string]int
}

func newGoldenVersions() *goldenVersions { return &goldenVersions{calls: map[string]int{}} }

func (v *goldenVersions) hit(method string) {
	v.mu.Lock()
	v.calls[method]++
	v.mu.Unlock()
}

func (v *goldenVersions) reset() {
	v.mu.Lock()
	v.calls = map[string]int{}
	v.mu.Unlock()
}

// snapshot returns the per-method read counts.
func (v *goldenVersions) snapshot() map[string]int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return maps.Clone(v.calls)
}

// goldenEntityVersions returns the fixed history of ref.
func goldenEntityVersions(ref entity.Ref) []store.VersionSnapshot {
	meta := func(n int, op store.VersionOp) store.VersionMeta {
		return store.VersionMeta{
			Version: n, Op: op, Type: "ticket", PrincipalUser: goldenAdmin,
			CreatedAt: goldenEpoch.Add(time.Duration(n) * time.Hour),
		}
	}
	switch ref.ID {
	case "TKT-1":
		return []store.VersionSnapshot{
			{VersionMeta: meta(1, store.VersionOpCreate), Content: "Alpha body.",
				Properties: map[string]any{"title": "Alpha", "status": "open", "secret": "first-secret"},
				Projection: []byte(`{}`)},
			{VersionMeta: meta(2, store.VersionOpUpdate), Content: "Alpha body.",
				Properties: map[string]any{"title": "Alpha", "status": "open", "secret": "alpha-secret"},
				Projection: []byte(`{}`)},
		}
	case "TKT-3":
		return []store.VersionSnapshot{
			{VersionMeta: meta(1, store.VersionOpCreate), Content: "Charlie, first draft.",
				Properties: map[string]any{"title": "Charlie", "status": "open", "secret": "charlie-secret"},
				Projection: []byte(`{}`)},
		}
	case "TKT-4":
		return []store.VersionSnapshot{
			{VersionMeta: meta(1, store.VersionOpCreate), Content: "Hidden child.",
				Properties: map[string]any{"title": "Delta", "status": "open", "secret": "delta-secret"},
				Projection: []byte(`{}`)},
		}
	}
	return nil
}

func (v *goldenVersions) ListVersions(_ context.Context, ref entity.Ref) ([]store.VersionMeta, error) {
	v.hit("ListVersions")
	snaps := goldenEntityVersions(ref)
	out := make([]store.VersionMeta, 0, len(snaps))
	for _, s := range snaps {
		m := s.VersionMeta
		m.Face = ref.Face
		out = append(out, m)
	}
	return out, nil
}

func (v *goldenVersions) GetVersion(_ context.Context, ref entity.Ref, version int) (*store.VersionSnapshot, error) {
	v.hit("GetVersion")
	snaps := goldenEntityVersions(ref)
	if version < 1 || version > len(snaps) {
		return nil, store.ErrNotFound
	}
	s := snaps[version-1]
	s.Face = ref.Face
	return &s, nil
}

// goldenRelationVersions returns the fixed history of the TKT-1 implements
// POL-1 edge, and nothing for any other edge.
func goldenRelationVersions(q store.RelationHistoryQuery) []store.RelationVersionSnapshot {
	if q.Key.From != "TKT-1" || q.Key.Type != "implements" || q.Key.To != "POL-1" {
		return nil
	}
	meta := func(n int, op store.VersionOp) store.RelationVersionMeta {
		return store.RelationVersionMeta{
			Version: n, Op: op, From: "TKT-1", Type: "implements", To: "POL-1",
			PrincipalUser: goldenAdmin, CreatedAt: goldenEpoch.Add(time.Duration(n) * time.Hour),
		}
	}
	return []store.RelationVersionSnapshot{
		{RelationVersionMeta: meta(1, store.VersionOpCreate), Projection: []byte(`{}`)},
		{RelationVersionMeta: meta(2, store.VersionOpUpdate), Projection: []byte(`{}`)},
	}
}

func (v *goldenVersions) ListRelationVersions(
	_ context.Context, q store.RelationHistoryQuery,
) ([]store.RelationVersionMeta, error) {
	v.hit("ListRelationVersions")
	snaps := goldenRelationVersions(q)
	out := make([]store.RelationVersionMeta, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, s.RelationVersionMeta)
	}
	return out, nil
}

func (v *goldenVersions) GetRelationVersion(
	_ context.Context, q store.RelationHistoryQuery, version int,
) (*store.RelationVersionSnapshot, error) {
	v.hit("GetRelationVersion")
	snaps := goldenRelationVersions(q)
	if version < 1 || version > len(snaps) {
		return nil, store.ErrNotFound
	}
	s := snaps[version-1]
	return &s, nil
}

func (v *goldenVersions) ListRelationLifetimes(
	_ context.Context, k entity.RelationKey,
) ([]store.RelationLifetime, error) {
	v.hit("ListRelationLifetimes")
	if k.From != "TKT-1" || k.Type != "implements" || k.To != "POL-1" {
		return nil, nil
	}
	return []store.RelationLifetime{{
		Lifetime: 1, RecordID: 1, VersionCount: 2, Live: true, FinalOp: store.VersionOpUpdate,
		FirstSeen: goldenEpoch.Add(time.Hour), LastSeen: goldenEpoch.Add(2 * time.Hour),
	}}, nil
}

// Writes are accepted and dropped: no corpus case reads back a version it
// wrote.
func (v *goldenVersions) WriteVersion(context.Context, store.VersionInput) error { return nil }

func (v *goldenVersions) WriteRelationVersion(context.Context, store.RelationVersionInput) error {
	return nil
}

// No corpus case purges history; a purge fails rather than pretend.
func (v *goldenVersions) PurgeVersions(context.Context, store.VersionPurgeRequest) (*store.PurgeResult, error) {
	return nil, errors.New("golden: version purge is not part of the corpus")
}

func (v *goldenVersions) PurgeRelationVersions(
	context.Context, store.RelationVersionPurgeRequest,
) (*store.PurgeResult, error) {
	return nil, errors.New("golden: version purge is not part of the corpus")
}

var _ store.VersionService = (*goldenVersions)(nil)
