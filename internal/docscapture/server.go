// Package docscapture implements the browser-backed screenshot capture for the
// rela-docs screenshot{} island (Tier B). It stands up the data-entry SPA over a
// seeded temp project and drives headless Chrome (chromedp) to capture a PNG.
//
// It is deliberately a SEPARATE package from internal/docs: it pulls in the whole
// data-entry + appbuild + chromedp dependency surface, which the core doc
// language must not carry. internal/docs depends on it only through the
// consumer-side docs.Capturer interface, injected by the CLI.
package docscapture

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/docs"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// roleHeader carries the requested ACL role from a chromedp navigation to the
// server's principal resolver, so one reused server can render different `as=`
// roles across screenshot{} islands (DR-S1).
const roleHeader = "X-Rela-Docs-Role"

// project holds a stood-up temp-project data-entry server: its temp dir, the
// appbuild services (closed on teardown), and the httptest server.
type project struct {
	dir string
	svc *appbuild.Services
	// scratchCleanup releases the throwaway BACKEND the temp project was
	// pointed at (a no-op on the fs build, a DROP SCHEMA on postgres). Separate
	// from removing dir, because on postgres the data does not live there.
	scratchCleanup func()
	server         *httptest.Server
	assignee       func(role string) principal.Principal
	// byRole is the role→user index behind assignee, kept so a caller can
	// distinguish an unknown role from a known one (see resolveRole).
	byRole map[string]string
	// seeded is how many seed ops have been applied to the store. The manual's
	// seed grows as create()/link() islands run; each screenshot{} passes the
	// FULL accumulated seed, so we apply only the new tail before capturing —
	// otherwise an entity created after the server stood up would be missing.
	seeded int
}

// syncSeed applies any seed ops beyond those already applied to the running
// store, so a screenshot{} of an entity created after standUp still renders.
//
// The watermark is POSITIONAL, which is only sound because docRuntime.seedOps is
// append-only: each call must pass the same prefix it passed last time, with new
// ops appended. If a caller ever rewrote or reordered the prefix, the already-
// applied ops would be skipped silently and every later assertion would be about
// a store that does not match the manual. Nothing in the type enforces this, so
// it is stated here.
func (p *project) syncSeed(ctx context.Context, seed []docs.SeedOp) error {
	if p.seeded > len(seed) {
		return fmt.Errorf("seed shrank from %d to %d ops: the seed must be append-only, "+
			"or already-applied ops are silently skipped", p.seeded, len(seed))
	}
	if p.seeded >= len(seed) {
		return nil
	}
	if err := docs.ApplySeedWith(
		p.seedCtx(ctx), p.svc.Store(), p.svc.EntityManager(), seed[p.seeded:],
	); err != nil {
		return err
	}
	p.seeded = len(seed)
	return nil
}

// seedCtx stamps the principal a seed replay writes as — the same default-role
// user standUp seeds with, so an `edit` arriving in a later island is
// authorized and attributed identically to one in the first batch.
func (p *project) seedCtx(ctx context.Context) context.Context {
	return principal.With(ctx, p.assignee(""))
}

// standUp copies the documented project's schema/config into a temp dir, seeds
// it by replaying the manual's create/link ops against the fsstore, and starts
// an httptest server for the SPA. The server's principal resolver maps the
// per-request role header to a principal assigned that role in acl.yaml.
// needSPA distinguishes the two callers: a screenshot renders the Vue app and
// cannot work without a built frontend, while an api{} assertion only reaches
// /api/v1 handlers and must NOT be blocked by a missing bundle — that is what
// lets api{} gate CI unconditionally.
func standUp(ctx context.Context, projectDir string, seed []docs.SeedOp, needSPA bool) (*project, error) {
	if needSPA {
		if err := dataentry.CheckEmbeddedSPA(); err != nil {
			return nil, fmt.Errorf("data-entry SPA not built (run `just build-frontend`): %w", err)
		}
	}

	tmp, err := os.MkdirTemp("", "rela-docscapture-*")
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(tmp, "project")
	//nolint:contextcheck // filesystem copy of static schema files is not request-scoped
	if cerr := copyProjectSchema(projectDir, dir); cerr != nil {
		_ = os.RemoveAll(tmp)
		return nil, cerr
	}

	// Point the temp project at a THROWAWAY backend before discovering it. On
	// the fs build that is what the temp dir already is; on postgres it is a
	// private scratch schema, without which the manual's fixture seed would be
	// written into the operator's live database.
	scratchOpts, scratchCleanup, err := scratchBackend(dir)
	if err != nil {
		_ = os.RemoveAll(tmp)
		return nil, err
	}

	// At, not Discover: this function BUILT `dir`, so the root is known.
	// Discovery walks UPWARD, and if the temp path happens to sit under a real
	// project it would resolve to that one — seeding the manual's fixtures into
	// a live database and taking its single-writer lock. Naming the root makes
	// the isolation structural rather than a property of where TMPDIR points
	// (TKT-SK2QQW).
	//
	//nolint:contextcheck // project construction is not request-scoped
	svc, err := appbuild.At(dir, script.NewEngine(), scratchOpts...)
	if err != nil {
		scratchCleanup()
		_ = os.RemoveAll(tmp)
		return nil, fmt.Errorf("open temp project: %w", err)
	}

	assignee := buildRoleAssignee(dir)
	roles := roleIndex(dir)

	// Seeding runs BEFORE the server exists, so it carries its own principal
	// rather than one resolved from a request header. The empty role is the
	// same default `as=` resolves to — a user with UPDATE grants — which is
	// what an `edit` op needs to get past the entitymanager's write gate, and
	// what it is then attributed to in the version history the manual shows.
	seedCtx := principal.With(ctx, assignee(""))

	// Seed the temp project's store. create/face/link go in RAW so automations
	// can't mutate the fixture; an `edit` goes through the entitymanager, which
	// is what makes it a real, versioned write (see docs.ApplySeedWith).
	if serr := docs.ApplySeedWith(seedCtx, svc.Store(), svc.EntityManager(), seed); serr != nil {
		svc.Close()
		scratchCleanup()
		_ = os.RemoveAll(tmp)
		return nil, fmt.Errorf("seed temp project: %w", serr)
	}

	app, err := dataentry.NewApp( //nolint:contextcheck // app construction is not request-scoped
		svc.FS(), svc.Paths(), svc.Meta(), svc.Store(), svc.Versions(),
		svc.EntityManager(), svc.Searcher(), svc.VisibleSearcher(), svc.ACL(),
		dataentry.NopFieldVerdictResolver{},
		svc.Audit(),
		svc.State(),
	)
	if err != nil {
		svc.Close()
		scratchCleanup()
		_ = os.RemoveAll(tmp)
		return nil, fmt.Errorf("build data-entry app: %w", err)
	}
	// Worlds, on the same terms as cmd/rela-server (main.go). Without this the
	// temp App refuses every `?world=` as an undeclared world — correct for a
	// deployment that never opted in, and exactly wrong for a screenshot of a
	// world-scoped page, which is the thing a worlds manual most needs to show.
	app.SetWorlds(appbuild.CompiledWorlds(svc))
	if err := dataentry.SetWorldNeighbors(app, svc.Store(), appbuild.RelationScopes(svc)); err != nil {
		svc.Close()
		scratchCleanup()
		_ = os.RemoveAll(tmp)
		return nil, fmt.Errorf("wire world neighbors: %w", err)
	}

	// Next-action wiring, on the same terms as cmd/rela-server (main.go).
	// Both are composition-root duties the dataentry package deliberately
	// does not perform for itself, and BOTH are load-bearing here: without
	// the user-state backend `/api/v1/_next_action` reports "not configured"
	// and answers a null suggestion to every request, so a manual figure of
	// the suggestion card would photograph an empty page and a page{} claim
	// about it would fail for a reason that has nothing to do with worlds.
	if err := app.SetUserState(svc.UserState()); err != nil {
		svc.Close()
		scratchCleanup()
		_ = os.RemoveAll(tmp)
		return nil, fmt.Errorf("wire next-action state: %w", err)
	}
	if err := app.SetNextActionMatchers(appbuild.NextActionMatchers); err != nil {
		svc.Close()
		scratchCleanup()
		_ = os.RemoveAll(tmp)
		return nil, fmt.Errorf("wire next-action matchers: %w", err)
	}

	// Per-request resolver: map the role header to a principal assigned that role.
	app.SetPrincipalResolver(func(r *http.Request) principal.Principal {
		return assignee(r.Header.Get(roleHeader))
	})

	p := &project{
		dir: tmp, svc: svc, scratchCleanup: scratchCleanup,
		assignee: assignee, byRole: roles, seeded: len(seed),
	}
	p.server = httptest.NewServer(app.NewRouter())
	return p, nil
}

func (p *project) close() {
	if p == nil {
		return
	}
	if p.server != nil {
		p.server.Close()
	}
	if p.svc != nil {
		p.svc.Close()
	}
	// After svc.Close: the store's pool must be torn down before the scratch
	// schema is dropped, or the DROP blocks on the store's own connections.
	if p.scratchCleanup != nil {
		p.scratchCleanup()
	}
	if p.dir != "" {
		_ = os.RemoveAll(p.dir)
	}
}

// copyProjectSchema copies the schema/config files the SPA needs (metamodel,
// data-entry config, acl, templates) into dst; entities are seeded separately.
func copyProjectSchema(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dst, ".rela"), 0o755); err != nil {
		return err
	}
	// Files and dirs that carry schema/config/presentation (NOT entities/relations).
	// Both schema names are listed; copyIfExists skips whichever is absent.
	// Literals rather than the project package constants: arch-lint forbids
	// docscapture -> project, and a local type here already binds the name.
	for _, name := range []string{
		"schema.yaml", "metamodel.yaml",
		"data-entry.yaml", "acl.yaml", "schedules.yaml",
	} {
		if err := copyIfExists(filepath.Join(src, name), filepath.Join(dst, name)); err != nil {
			return err
		}
	}
	// Presentation/behavior dirs the SPA + data-entry config reference (NOT
	// entities/relations, which are seeded separately).
	for _, d := range []string{"templates", "scripts", "actions", "apps", "documents"} {
		if err := copyDirIfExists(filepath.Join(src, d), filepath.Join(dst, d)); err != nil {
			return err
		}
	}
	return nil
}

// copyIfExists copies src to dst, treating a missing src as a no-op.
//
// Both paths are built by copyProjectSchema from a fixed set of literal
// filenames: dst under a temp dir this process just created, src under the
// operator's --project root. No caller- or manual-supplied string reaches
// either, so there is no traversal surface here.
func copyIfExists(src, dst string) error {
	data, err := os.ReadFile(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// #nosec G703 -- dst is filepath.Join(<os.MkdirTemp dir>, <literal name>);
	// the only non-constant part is a temp dir this process created.
	return os.WriteFile(dst, data, 0o644)
}

func copyDirIfExists(src, dst string) error {
	info, err := os.Stat(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	// Shell cp -r keeps this simple and matches the e2e test's approach.
	if out, cerr := exec.CommandContext(context.Background(), "cp", "-r", src, dst).CombinedOutput(); cerr != nil {
		return fmt.Errorf("copy %s: %w: %s", src, cerr, out)
	}
	return nil
}

// roleIndex returns the role→user index for the project's acl.yaml, so a caller
// can validate a requested role name. Empty when there is no policy, in which
// case no validation is possible and none is done.
func roleIndex(projectDir string) map[string]string {
	byRole := map[string]string{}
	pol, err := acl.LoadPolicy(filepath.Join(projectDir, "acl.yaml"))
	if err != nil {
		return byRole
	}
	for user, role := range pol.Assignments {
		if _, seen := byRole[role]; !seen {
			byRole[role] = user
		}
	}
	return byRole
}

// buildRoleAssignee returns a function mapping a requested role name to a
// principal assigned that role in acl.yaml. acl.yaml Assignments are User→role;
// we invert to role→a User holding it. When the requested role is empty or
// unknown, it falls back to any assigned user (so a real Declarative ACL admits
// the SPA's reads — an unstamped "unknown" principal is rejected). When there is
// no acl.yaml / no assignments, any stamped user is fine (NopACL admits all).
func buildRoleAssignee(projectDir string) func(role string) principal.Principal {
	byRole := map[string]string{}
	var defaultUser string
	if pol, err := acl.LoadPolicy(filepath.Join(projectDir, "acl.yaml")); err == nil {
		for user, role := range pol.Assignments {
			if _, seen := byRole[role]; !seen {
				byRole[role] = user
			}
		}
		defaultUser = pickDefaultUser(pol, byRole)
	}
	if defaultUser == "" {
		defaultUser = "docs-capture"
	}
	return func(role string) principal.Principal {
		p, _ := resolveRole(byRole, defaultUser, role)
		return p
	}
}

// pickDefaultUser chooses the principal an empty `as=` resolves to: a user
// holding the WIDEST update grant, chosen DETERMINISTICALLY.
//
// Both properties are load-bearing, and neither was true before. Ranging a map
// picked an arbitrary role, so a project with two updating roles resolved the
// default principal differently between runs of the same build. A narrow role
// (`update: [guide]`) could then be chosen for a screenshot of a policy form —
// which renders read-only rather than editable — or for a seeded `edit`, which
// the entitymanager refuses outright. Both failures are intermittent, and
// neither error names the cause.
//
// A grant of `*` wins over any explicit list; ties break on the role name.
// Returns "" when no assigned role can update, leaving the caller's fallback.
func pickDefaultUser(pol *acl.Policy, byRole map[string]string) string {
	var best string
	for _, name := range sortedRoleNames(pol.Roles) {
		role := pol.Roles[name]
		if len(role.Update) == 0 {
			continue
		}
		u, ok := byRole[name]
		if !ok {
			continue
		}
		if grantsAllTypes(role.Update) {
			return u
		}
		if best == "" {
			best = u
		}
	}
	if best != "" {
		return best
	}
	// Stable fallback: any assigned user, but always the same one.
	names := make([]string, 0, len(byRole))
	for r := range byRole {
		names = append(names, r)
	}
	sort.Strings(names)
	if len(names) > 0 {
		return byRole[names[0]]
	}
	return ""
}

// resolveRole maps a requested role to a principal and reports whether the role
// was KNOWN. The bool is the whole point: an unknown role silently resolves to
// defaultUser — a user chosen for having UPDATE grants — so `as="vewer"` runs as
// the editor and an assertion passes for entirely the wrong reason. Callers that
// know the role was explicitly named (api{}) refuse an unknown one; the empty
// `as=` default stays a legitimate fallback.
func resolveRole(byRole map[string]string, defaultUser, role string) (principal.Principal, bool) {
	user := defaultUser
	known := true
	if role != "" {
		u, ok := byRole[role]
		if ok {
			user = u
		} else {
			known = false
		}
	}
	return principal.Principal{User: user, Tool: principal.ToolDataEntry}, known
}

// knownRoles lists the roles that have an assigned user, for a failure message.
// requireKnownRole refuses an `as=` naming a role no principal in acl.yaml
// holds. The assignee resolves an unknown role to a privileged default user,
// so a typo'd role would run the island as the editor and its assertion would
// pass for the wrong reason — the vacuous-pass shape every island (api{},
// screenshot{}, page{}) must refuse alike. Only checked when the project HAS
// assignments to check against; an empty `as` is the default principal.
func (p *project) requireKnownRole(as string) error {
	if as == "" || len(p.byRole) == 0 {
		return nil
	}
	if _, ok := p.byRole[as]; ok {
		return nil
	}
	return fmt.Errorf(
		"as=%q: no principal is assigned that role in acl.yaml, and an unknown role "+
			"falls back to a privileged default — so this request would run as someone "+
			"else. Known roles: %s", as, strings.Join(knownRoles(p.byRole), ", "))
}

func knownRoles(byRole map[string]string) []string {
	out := make([]string, 0, len(byRole))
	for r := range byRole {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// hasChrome reports whether a Chrome/Chromium binary is resolvable, so the
// caller can fail loud (no graceful degradation) before launching.
func hasChrome() (string, bool) {
	for _, name := range chromeNames {
		if p, err := exec.LookPath(name); err == nil {
			return p, true
		}
	}
	// macOS default install location (not on PATH).
	for _, p := range chromePaths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
}

var chromeNames = []string{
	"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome",
}

var chromePaths = []string{
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
}

// formURL builds the SPA edit-form route for a capture spec. Only view="form"
// is supported (validated in the resolver); the form id defaults to edit_<type>.
// captureURL builds the SPA url for one capture.
//
// `view` selects the screen kind, matching the SPA's own routes: a manual that
// wants to show a list, a detail page, a create form and a search result needs
// four different urls, and deriving them here keeps the doc language spelling
// them as a VIEW KIND rather than as hand-written paths that would rot with the
// router.
//
// The world rides as `?world=` — the same query parameter a user's browser
// carries — so a screenshot is literally the page a reader would see, not a
// specially-rendered variant.
func captureURL(base string, spec docs.CaptureSpec) string {
	var path string
	switch spec.View {
	case "list":
		path = "/list/" + spec.List
	case "entity":
		path = fmt.Sprintf("/entity/%s/%s", spec.Type, spec.Entity)
	case "search":
		path = "/search"
	case "dashboard":
		path = "/dashboard"
	case "analyze":
		path = "/analyze"
	case "kanban":
		path = "/kanban/" + spec.List
	case "calendar":
		path = "/calendar/" + spec.List
	case "history":
		path = fmt.Sprintf("/history/%s/%s", spec.Type, spec.Entity)
	case "create":
		// A create form has no entity id yet — that is the whole difference
		// between it and an edit form, and it is the screen where a world
		// matters most (the new entity's face may not exist in the world the
		// list was showing).
		form := spec.Form
		if form == "" {
			form = "new_" + spec.Type
		}
		path = "/form/" + form
	default: // "form" — edit an existing entity
		form := spec.Form
		if form == "" {
			form = "edit_" + spec.Type
		}
		path = fmt.Sprintf("/form/%s/%s", form, spec.Entity)
	}

	q := url.Values{}
	if spec.World != "" {
		q.Set("world", spec.World)
	}
	if spec.Query != "" {
		q.Set("q", spec.Query)
	}
	if len(q) == 0 {
		return base + path
	}
	return base + path + "?" + q.Encode()
}

func trimSlash(s string) string { return strings.TrimRight(s, "/") }

// sortedRoleNames lists role names in a stable order, so the default-principal
// choice cannot vary between runs of the same build.
func sortedRoleNames(roles map[string]acl.RoleDef) []string {
	out := make([]string, 0, len(roles))
	for name := range roles {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// grantsAllTypes reports whether a grant list is the unrestricted `*`.
func grantsAllTypes(grants []string) bool {
	return slices.Contains(grants, "*")
}

// countVersions reports how many versions the history API lists for a capture's
// entity, through the SAME server and role the screenshot will render as — so
// what the wait observes is exactly what the page will show, rather than a
// privileged or unscoped count that could disagree with the figure.
//
// A non-200 is reported as an error rather than as zero: "history is
// unavailable" and "history is empty" are different answers, and silently
// treating the first as the second would spin until the deadline and then blame
// the sweep for a backend that never had the capability.
func (p *project) countVersions(ctx context.Context, spec docs.CaptureSpec) (int, error) {
	u := fmt.Sprintf("%s/api/v1/_history/%s/%s",
		trimSlash(p.server.URL), url.PathEscape(spec.Type), url.PathEscape(spec.Entity))
	if spec.World != "" {
		u += "?world=" + url.QueryEscape(spec.World)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return 0, err
	}
	if spec.As != "" {
		req.Header.Set(roleHeader, spec.As)
	}
	resp, err := p.server.Client().Do(req)
	if err != nil {
		return 0, fmt.Errorf("reading version history for %s: %w", spec.Entity, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotImplemented {
		// The backend has no version history at all. Naming that plainly is the
		// whole point: the alternative is waiting out the deadline and then
		// blaming a debounce that was never the problem. It is also the one
		// error an author is most likely to hit, by building a manual with
		// await_versions on the default (filesystem) build.
		return 0, fmt.Errorf(
			"version history is not available on this storage backend, so screenshot{"+
				"view=\"history\", await_versions=...} for %s cannot be satisfied. Version "+
				"history is a POSTGRES capability (only pgstore implements it) — build this "+
				"manual with the postgres binary, e.g. `just docs-visual-postgres`", spec.Entity)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("reading version history for %s: HTTP %d", spec.Entity, resp.StatusCode)
	}
	var body struct {
		Versions []struct{} `json:"versions"`
	}
	if derr := json.NewDecoder(resp.Body).Decode(&body); derr != nil {
		return 0, fmt.Errorf("decoding version history for %s: %w", spec.Entity, derr)
	}
	return len(body.Versions), nil
}
