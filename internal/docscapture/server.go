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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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
	dir      string
	svc      *appbuild.Services
	server   *httptest.Server
	assignee func(role string) principal.Principal
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
	if err := docs.ApplySeed(ctx, p.svc.Store(), seed[p.seeded:]); err != nil {
		return err
	}
	p.seeded = len(seed)
	return nil
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

	svc, err := appbuild.Discover(dir, script.NewEngine()) //nolint:contextcheck // discovery is not request-scoped
	if err != nil {
		_ = os.RemoveAll(tmp)
		return nil, fmt.Errorf("discover temp project: %w", err)
	}

	// Seed the temp project's store (raw — no entitymanager, so automations can't
	// mutate the fixture); the same ops already ran against the in-mem store.
	if serr := docs.ApplySeed(ctx, svc.Store(), seed); serr != nil {
		svc.Close() //nolint:contextcheck // teardown is not request-scoped; Close takes no ctx
		_ = os.RemoveAll(tmp)
		return nil, fmt.Errorf("seed temp project: %w", serr)
	}

	assignee := buildRoleAssignee(dir)
	roles := roleIndex(dir)

	app, err := dataentry.NewApp( //nolint:contextcheck // app construction is not request-scoped
		svc.FS(), svc.Paths(), svc.Meta(), svc.Store(), svc.Versions(),
		svc.EntityManager(), svc.Searcher(), svc.VisibleSearcher(), svc.ACL(),
		dataentry.NopFieldVerdictResolver{},
		svc.Audit(),
		svc.State(),
	)
	if err != nil {
		svc.Close() //nolint:contextcheck // teardown is not request-scoped; Close takes no ctx
		_ = os.RemoveAll(tmp)
		return nil, fmt.Errorf("build data-entry app: %w", err)
	}
	// Per-request resolver: map the role header to a principal assigned that role.
	app.SetPrincipalResolver(func(r *http.Request) principal.Principal {
		return assignee(r.Header.Get(roleHeader))
	})

	p := &project{dir: tmp, svc: svc, assignee: assignee, byRole: roles, seeded: len(seed)}
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
		// The empty-`as=` default prefers a role that can UPDATE (so an edit form
		// renders editable, not the read-only "not editable" state). Falls back to
		// any assigned user, then a placeholder.
		for name, role := range pol.Roles {
			if len(role.Update) > 0 {
				if u, ok := byRole[name]; ok {
					defaultUser = u
					break
				}
			}
		}
		if defaultUser == "" {
			for _, u := range byRole {
				defaultUser = u
				break
			}
		}
	}
	if defaultUser == "" {
		defaultUser = "docs-capture"
	}
	return func(role string) principal.Principal {
		p, _ := resolveRole(byRole, defaultUser, role)
		return p
	}
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
func formURL(base string, spec docs.CaptureSpec) string {
	form := spec.Form
	if form == "" {
		form = "edit_" + spec.Type
	}
	return fmt.Sprintf("%s/form/%s/%s", base, form, spec.Entity)
}

func trimSlash(s string) string { return strings.TrimRight(s, "/") }
