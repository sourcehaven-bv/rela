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
// tenant config can load, read from the shared data-entry.yaml. A
// multi-tenant server shows its "setting up" screen from this shell, so
// without the name and language that screen reads "rela" and follows the
// browser's language (BUG-21N3J4N). The pages for a suspended or archived
// workspace come from the same shell, so the logo, the account links and the
// contact text they show come from here too. The zero value leaves the shell
// as built.
//
// Every value is operator configuration, which is not a secret (see the
// root CLAUDE.md), and is the same for every tenant of the server.
type ShellBoot struct {
	AppName  string
	Language string
	// Logo is `app.logo`.
	Logo string
	// SignOut and Account are `account.sign_out` and `account.account`.
	SignOut string
	Account string
	// Contact is `account.contact`.
	Contact string
}

// ShellBootFrom reads a [ShellBoot] from the bytes of data-entry.yaml. Only
// the `app:` and `account:` blocks are read. A value that fails the config
// load's own check is dropped, so a typo there cannot break the shell; the
// full config load reports it.
func ShellBootFrom(data []byte) (ShellBoot, error) {
	var doc struct {
		App     dataentryconfig.AppConfig      `yaml:"app"`
		Account *dataentryconfig.AccountConfig `yaml:"account"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return ShellBoot{}, err
	}
	return shellBootOf(doc.App, doc.Account), nil
}

// shellBootOf is the [ShellBoot] for an `app:` and an `account:` block.
// A language that is not a plain BCP 47 tag and a link that is not a valid
// account link are dropped.
func shellBootOf(app dataentryconfig.AppConfig, acc *dataentryconfig.AccountConfig) ShellBoot {
	b := ShellBoot{AppName: strings.TrimSpace(app.Name), Logo: shellLink(app.Logo)}
	if lang := strings.TrimSpace(app.Language); languageTag.MatchString(lang) {
		b.Language = lang
	}
	if acc != nil {
		b.SignOut = shellLink(acc.SignOut)
		b.Account = shellLink(acc.Account)
		if c := strings.TrimSpace(acc.Contact); len([]rune(c)) <= dataentryconfig.MaxAccountContactRunes {
			b.Contact = c
		}
	}
	return b
}

// shellLink is s when it is a valid account link, else empty.
func shellLink(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || !dataentryconfig.ValidAccountLink(s) {
		return ""
	}
	return s
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
	metaLogo     = "rela-app-logo"
	metaSignOut  = "rela-sign-out"
	metaAccount  = "rela-account-url"
	metaContact  = "rela-contact"
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
	for _, m := range []struct{ name, value string }{
		{metaLogo, boot.Logo},
		{metaSignOut, boot.SignOut},
		{metaAccount, boot.Account},
		{metaContact, boot.Contact},
	} {
		if m.value != "" {
			s = insertBefore(s, "</head>", metaTag(m.name, html.EscapeString(m.value)))
		}
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
