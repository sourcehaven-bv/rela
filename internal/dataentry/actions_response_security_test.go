package dataentry

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// TestAction_RichResponseRefusesActiveContentType attacks the content-type
// allowlist through the REAL handler rather than the parser.
//
// The rich body is attacker-influenceable output served same-origin with the
// SPA, so an active-content type would be a scripting sink. nosniff and the
// sandbox CSP are what make a mistake here survivable; the allowlist is what
// means a single header regression is not immediately exploitable. Both have
// to hold, so both are tested — this is the allowlist half.
func TestAction_RichResponseRefusesActiveContentType(t *testing.T) {
	for _, ct := range []string{
		"text/html",
		"image/svg+xml",
		"application/xhtml+xml",
		"text/html; charset=utf-8",
	} {
		t.Run(ct, func(t *testing.T) {
			app := newActionTestApp(t, map[string]string{
				"xss.lua": `return {status = 200, body = "<script>alert(1)</script>", ` +
					`content_type = "` + ct + `"}`,
			})
			app.Cfg().Actions = map[string]dataentryconfig.Action{
				"xss": {Script: "xss.lua", Request: &dataentryconfig.ActionRequest{Body: true}},
			}

			rec := postRequestAction(t, app, "xss", `{}`, nil)
			if rec.Code == http.StatusOK {
				t.Fatalf("%q was served with 200; the allowlist must refuse it", ct)
			}
			got := rec.Header().Get("Content-Type")
			if strings.Contains(got, "html") || strings.Contains(got, "svg") {
				t.Fatalf("an active content type reached the wire: %q", got)
			}
		})
	}
}

// TestAction_ContentTypeCannotSplitHeaders attacks header injection through
// content_type. mime.ParseMediaType refuses CR/LF, so the smuggled name never
// reaches w.Header().Set — but net/http would also refuse it, and relying on
// that alone would leave the guarantee somewhere nobody can see it.
func TestAction_ContentTypeCannotSplitHeaders(t *testing.T) {
	for _, ct := range []string{
		`text/plain\r\nX-Injected: 1`,
		`text/plain\nX-Injected: 1`,
		`text/plain;\r\n\tX-Injected: 1`,
	} {
		t.Run(ct, func(t *testing.T) {
			app := newActionTestApp(t, map[string]string{
				// The Lua literal carries real CR/LF via the escape sequences.
				"split.lua": `return {status = 200, body = "x", content_type = "` + ct + `"}`,
			})
			app.Cfg().Actions = map[string]dataentryconfig.Action{
				"split": {Script: "split.lua", Request: &dataentryconfig.ActionRequest{Body: true}},
			}

			rec := postRequestAction(t, app, "split", `{}`, nil)
			if rec.Header().Get("X-Injected") != "" {
				t.Fatal("content_type smuggled a second header")
			}
			if rec.Code == http.StatusOK {
				t.Fatalf("a CR/LF-bearing content_type was accepted (status %d)", rec.Code)
			}
		})
	}
}

// TestAction_RichBodyIsNotHTMLInterpretable pins the SECOND half of the
// defense: even for an ALLOWED type, the response carries nosniff and a
// sandbox CSP, so a body full of markup cannot become active content if a
// browser is pointed at the endpoint directly.
func TestAction_RichBodyIsNotHTMLInterpretable(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"markup.lua": `return {status = 200, body = "<img src=x onerror=alert(1)>", ` +
			`content_type = "text/plain"}`,
	})
	app.Cfg().Actions = map[string]dataentryconfig.Action{
		"markup": {Script: "markup.lua", Request: &dataentryconfig.ActionRequest{Body: true}},
	}

	rec := postRequestAction(t, app, "markup", `{}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// The body is served VERBATIM — this endpoint does not sanitize, and must
	// not pretend to. The headers are the control.
	if rec.Body.String() != "<img src=x onerror=alert(1)>" {
		t.Fatalf("body was altered: %q", rec.Body.String())
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("nosniff missing: a browser could re-interpret text/plain markup as HTML")
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "sandbox") {
		t.Errorf("sandbox CSP missing, got %q", csp)
	}
}

// TestAction_SPAResponseIsUnhardened states the deliberate asymmetry: the SPA
// envelope path does NOT get the download-hardening headers, because it is a
// rela-generated JSON envelope the SPA reads, not script-controlled bytes.
// Adding them there would be cargo-culting, and would change the caching
// behavior of every existing action.
func TestAction_SPAResponseIsUnhardened(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"toast.lua": `return {message = "saved", message_type = "success"}`,
	})
	app.Cfg().Actions = map[string]dataentryconfig.Action{
		"toast": {Script: "toast.lua"},
	}

	rec := postRequestAction(t, app, "toast", `{}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "" {
		t.Errorf("the SPA envelope path gained a CSP it did not have: %q", got)
	}
	if !strings.Contains(rec.Body.String(), `"message":"saved"`) {
		t.Errorf("SPA envelope changed shape: %s", rec.Body.String())
	}
}
