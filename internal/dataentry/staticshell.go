package dataentry

import (
	"context"
	"errors"
	"html"
	"io/fs"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// StaticShell serves the embedded SPA shell and its static assets and
// nothing else: the `/static/` mount and the shell catch-all of
// [App.NewRouter], without an App.
//
// A multi-tenant host serves it to a request that has no tenant yet, so a
// browser can load and render a signed-out state before it presents an
// assertion. Every byte it can return is compiled into the binary, so it
// reads no store and no project file and cannot disclose a tenant's data,
// whichever path it is handed. The operator's `custom/` injection is left
// out because it reads project files; a request with a verified tenant gets
// the tenant's full router instead.
//
// boot names the shared project's app in the shell (see [ShellBoot]). It is
// read once at startup from the operator's config, not per request, so the
// handler still reads no project file and no store.
//
// Panics only if the binary was built without the embedded SPA filesystem,
// which [CheckEmbeddedSPA] reports at startup.
//
// A headless build has no shell, so it serves the same root as
// [App.NewRouter] does there: a text pointer to the API at `/`, 404 elsewhere.
func StaticShell(boot ShellBoot) http.Handler {
	if Headless {
		return withSpecLink(headlessRoot())
	}
	staticFS, err := fs.Sub(staticFiles, "static")
	// coverage-ignore-start: panic-invariant: fs.Sub on the compile-time embed.FS with a literal path cannot fail
	if err != nil {
		panic("failed to mount embedded static filesystem (static): " + err.Error())
	}
	spaFS, err := fs.Sub(staticFiles, "static/v2")
	if err != nil {
		panic("failed to mount embedded SPA filesystem (static/v2): " + err.Error())
	}
	// coverage-ignore-end
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.Handle("/", bootShellHandler(spaFS, boot))
	return mux
}

// ShellBoot is what the tenant-free shell says about the app before any
// tenant config can load: the `app.name` and `app.language` of the shared
// data-entry.yaml. A multi-tenant server shows its "setting up" screen from
// this shell, so without it that screen reads "rela" and follows the
// browser's language (BUG-21N3J4N). The zero value leaves the shell as built.
//
// Both values are operator configuration, which is not a secret (see the
// root CLAUDE.md), and are the same for every tenant of the server.
type ShellBoot struct {
	AppName  string
	Language string
}

// ShellBootFrom reads a [ShellBoot] from the bytes of data-entry.yaml. Only
// the `app:` block is read. A language that is not a plain BCP 47 tag is
// dropped, so a typo there cannot break the shell; the full config load
// reports it.
func ShellBootFrom(data []byte) (ShellBoot, error) {
	var doc struct {
		App struct {
			Name     string `yaml:"name"`
			Language string `yaml:"language"`
		} `yaml:"app"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return ShellBoot{}, err
	}
	b := ShellBoot{AppName: strings.TrimSpace(doc.App.Name)}
	if lang := strings.TrimSpace(doc.App.Language); languageTag.MatchString(lang) {
		b.Language = lang
	}
	return b, nil
}

// shellBoot is the [ShellBoot] of the App's current config. A language that
// is not a plain BCP 47 tag is dropped, as in [ShellBootFrom].
func (a *App) shellBoot() ShellBoot {
	app := a.State().Cfg.App
	b := ShellBoot{AppName: strings.TrimSpace(app.Name)}
	if lang := strings.TrimSpace(app.Language); languageTag.MatchString(lang) {
		b.Language = lang
	}
	return b
}

// LoadShellBoot reads a [ShellBoot] from data-entry.yaml through load, a
// project config loader's Load. A project without the file has the zero
// value and no error.
func LoadShellBoot(ctx context.Context, load func(context.Context, string) ([]byte, error)) (ShellBoot, error) {
	data, err := load(ctx, dataentryconfig.ConfigFile)
	if errors.Is(err, fs.ErrNotExist) {
		return ShellBoot{}, nil
	}
	if err != nil {
		return ShellBoot{}, err
	}
	return ShellBootFrom(data)
}

// languageTag is a loose BCP 47 shape: letters and digits in hyphenated
// parts. It keeps anything else out of the lang attribute.
var languageTag = regexp.MustCompile(`^[A-Za-z]{2,8}(-[A-Za-z0-9]{1,8})*$`)

// The meta names the SPA reads at boot (frontend/src/i18n/bootMeta.ts).
const (
	metaAppName  = "rela-app-name"
	metaLanguage = "rela-language"
)

// bootShell writes boot into an index.html: the title, the html lang
// attribute, and a meta per value for the SPA. Values are HTML-escaped. A
// missing insertion point leaves that part as it was.
func bootShell(shell []byte, boot ShellBoot) []byte {
	s := string(shell)
	if boot.AppName != "" {
		name := html.EscapeString(boot.AppName)
		if i := strings.Index(s, "<title>"); i >= 0 {
			if j := strings.Index(s[i:], "</title>"); j >= 0 {
				s = s[:i] + "<title>" + name + s[i+j:]
			}
		}
		s = insertBefore(s, "</head>", metaTag(metaAppName, name))
	}
	if boot.Language != "" {
		lang := html.EscapeString(boot.Language)
		s = langAttr.ReplaceAllLiteralString(s, `<html lang="`+lang+`"`)
		s = insertBefore(s, "</head>", metaTag(metaLanguage, lang))
	}
	return []byte(s)
}

var langAttr = regexp.MustCompile(`<html lang="[^"]*"`)

func metaTag(name, escapedContent string) string {
	return `<meta name="` + name + `" content="` + escapedContent + `" />`
}

// bootShellHandler serves fsys like [spaHandler], with the shell rewritten
// by [bootShell]. The rewrite is done once, on the first shell request.
func bootShellHandler(fsys fs.FS, boot ShellBoot) http.Handler {
	fallback := spaHandler(fsys)
	if boot == (ShellBoot{}) {
		return fallback
	}
	body := sync.OnceValue(func() []byte {
		shell, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			return nil
		}
		return bootShell(shell, boot)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := body()
		if b == nil || !servesSPAShell(fsys, r.URL.Path) {
			fallback.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Content-Length", strconv.Itoa(len(b)))
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(b)
	})
}
