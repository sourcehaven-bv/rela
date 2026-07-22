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
}

// standUp copies the documented project's schema/config into a temp dir, seeds
// it by replaying the manual's create/link ops against the fsstore, and starts
// an httptest server for the SPA. The server's principal resolver maps the
// per-request role header to a principal assigned that role in acl.yaml.
func standUp(ctx context.Context, projectDir string, seed []docs.SeedOp) (*project, error) {
	if err := dataentry.CheckEmbeddedSPA(); err != nil {
		return nil, fmt.Errorf("data-entry SPA not built (run `just build-frontend`): %w", err)
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
		svc.Close()
		_ = os.RemoveAll(tmp)
		return nil, fmt.Errorf("seed temp project: %w", serr)
	}

	assignee := buildRoleAssignee(dir)

	app, err := dataentry.NewApp( //nolint:contextcheck // app construction is not request-scoped
		svc.FS(), svc.Paths(), svc.Meta(), svc.Store(),
		svc.EntityManager(), svc.Searcher(), svc.VisibleSearcher(), svc.ACL(),
		dataentry.NopFieldVerdictResolver{},
		svc.Audit(),
	)
	if err != nil {
		svc.Close()
		_ = os.RemoveAll(tmp)
		return nil, fmt.Errorf("build data-entry app: %w", err)
	}
	// Per-request resolver: map the role header to a principal assigned that role.
	app.SetPrincipalResolver(func(r *http.Request) principal.Principal {
		return assignee(r.Header.Get(roleHeader))
	})

	p := &project{dir: tmp, svc: svc, assignee: assignee}
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
	for _, name := range []string{"metamodel.yaml", "data-entry.yaml", "acl.yaml", "schedules.yaml"} {
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

func copyIfExists(src, dst string) error {
	data, err := os.ReadFile(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
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
		user := defaultUser
		if role != "" {
			if u, ok := byRole[role]; ok {
				user = u
			}
		}
		return principal.Principal{User: user, Tool: principal.ToolDataEntry}
	}
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

// formURL builds the SPA route for a capture spec against the given base URL.
func formURL(base string, spec docs.CaptureSpec) string {
	switch spec.View {
	case "entity":
		return fmt.Sprintf("%s/entity/%s/%s", base, spec.Type, spec.Entity)
	case "list":
		return fmt.Sprintf("%s/list/%s", base, firstNonEmpty(spec.Form, spec.Type))
	default: // "form"
		form := spec.Form
		if form == "" {
			form = "edit_" + spec.Type
		}
		return fmt.Sprintf("%s/form/%s/%s", base, form, spec.Entity)
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func trimSlash(s string) string { return strings.TrimRight(s, "/") }
