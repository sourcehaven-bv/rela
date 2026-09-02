package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// postAction is the shared driver for the request-scoped action tests.
func postRequestAction(t *testing.T, app *App, id, body string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	callAction(app, req, rec)
	return rec
}

// TestAction_RequestAbsentByDefault pins the opt-out default: an action with no
// request: block sees no rela.request at all, even when a body was posted.
// Request-scoped execution is opt-in, and this is what makes that true rather
// than merely documented.
func TestAction_RequestAbsentByDefault(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"probe.lua": `return {message = rela.request == nil and "absent" or "present"}`,
	})
	app.Cfg().Actions = map[string]dataentryconfig.Action{
		"probe": {Script: "probe.lua"},
	}

	rec := postRequestAction(t, app, "probe", `{"secret":"x"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp v1.ActionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Message != "absent" {
		t.Fatalf("rela.request must be absent without a request: block, got %q", resp.Message)
	}
}

// TestAction_RequestBodyAndQuery covers the happy path an Icinga-style hook
// needs: a JSON body, the query string, and a rich response.
func TestAction_RequestBodyAndQuery(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"alert.lua": `
			local host = rela.request.body.host
			local dry  = rela.request.query.dry_run
			return {
				status = 202,
				body = host .. "/" .. dry .. "/" .. rela.request.method,
				content_type = "text/plain",
			}
		`,
	})
	app.Cfg().Actions = map[string]dataentryconfig.Action{
		"alert": {Script: "alert.lua", Request: &dataentryconfig.ActionRequest{Body: true, Query: true}},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/alert?dry_run=1", strings.NewReader(`{"host":"web1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	callAction(app, req, rec)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "web1/1/POST" {
		t.Fatalf("body = %q", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("content-type = %q", ct)
	}
}

// TestAction_RichResponseIsHardened pins the export/attachment hardening
// recipe on a script-controlled body: it is attacker-influenced output served
// same-origin with the SPA.
func TestAction_RichResponseIsHardened(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"raw.lua": `return {status = 200, body = "<script>alert(1)</script>", content_type = "text/plain"}`,
	})
	app.Cfg().Actions = map[string]dataentryconfig.Action{
		"raw": {Script: "raw.lua", Request: &dataentryconfig.ActionRequest{Body: true}},
	}

	rec := postRequestAction(t, app, "raw", `{}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	for header, want := range map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "sandbox; default-src 'none'",
		"Cache-Control":           "no-store",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

// TestAction_HeadersAreAllowlisted pins that only the configured headers reach
// the script, and that an unlisted one is absent rather than empty.
func TestAction_HeadersAreAllowlisted(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"peek.lua": `
			local h = rela.request.headers
			return {
				status = 200,
				body = (h["x-event-type"] or "nil") .. "|" .. (h["x-secret"] or "nil")
					.. "|" .. (h["authorization"] or "nil"),
				content_type = "text/plain",
			}
		`,
	})
	app.Cfg().Actions = map[string]dataentryconfig.Action{
		"peek": {
			Script:  "peek.lua",
			Request: &dataentryconfig.ActionRequest{Headers: []string{"X-Event-Type"}},
		},
	}

	rec := postRequestAction(t, app, "peek", `{}`, func(r *http.Request) {
		r.Header.Set("X-Event-Type", "alert")
		r.Header.Set("X-Secret", "leaked")
		r.Header.Set("Authorization", "Bearer hunter2")
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "alert|nil|nil" {
		t.Fatalf("headers leaked past the allowlist: %q", got)
	}
}

// TestAction_BodyCap pins the size cap the action endpoint had none of. An
// oversized body is REFUSED with a 413, never truncated: a truncated JSON body
// usually fails to parse, but a truncated form body parses fine and would have
// the script act on quietly-wrong data.
func TestAction_BodyCap(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"echo.lua": `return {status = 200, body = "ok", content_type = "text/plain"}`,
	})
	app.Cfg().Actions = map[string]dataentryconfig.Action{
		"echo": {
			Script:  "echo.lua",
			Request: &dataentryconfig.ActionRequest{Body: true, MaxBodyBytes: 32},
		},
	}

	big := `{"pad":"` + strings.Repeat("a", 64) + `"}`
	rec := postRequestAction(t, app, "echo", big, nil)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "ok") {
		t.Fatal("script must not have run on an oversized body")
	}
}

// TestAction_ScriptErrorBeatsScriptStatus states the error-mapping rule
// explicitly: a script that FAILS gets rela's error envelope, whatever status
// it might have been about to return. Only a successful return chooses a
// status. A failed script has, by definition, not decided anything.
func TestAction_ScriptErrorBeatsScriptStatus(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"boom.lua": `error("kaboom")`,
	})
	app.Cfg().Actions = map[string]dataentryconfig.Action{
		"boom": {Script: "boom.lua", Request: &dataentryconfig.ActionRequest{Body: true}},
	}

	rec := postRequestAction(t, app, "boom", `{}`, nil)
	if rec.Code == http.StatusOK || rec.Code == http.StatusAccepted {
		t.Fatalf("a failing script must not produce a success status, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "json") {
		t.Fatalf("expected a rela JSON error envelope, got content-type %q", ct)
	}
}

// TestAction_RichResponseAllowsErrorStatus pins that a SUCCESSFUL script return
// may still choose a 4xx/5xx — the case that lets a producer be told "your
// payload was unusable" without rela inventing an envelope for it.
func TestAction_RichResponseAllowsErrorStatus(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"reject.lua": `return {status = 422, body = "bad host", content_type = "text/plain"}`,
	})
	app.Cfg().Actions = map[string]dataentryconfig.Action{
		"reject": {Script: "reject.lua", Request: &dataentryconfig.ActionRequest{Body: true}},
	}

	rec := postRequestAction(t, app, "reject", `{}`, nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "bad host" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

// TestAction_MalformedBodyDoesNotFailTheRequest pins that a non-JSON body is
// reachable as rela.request.raw with body left nil, rather than 400ing.
// A request-scoped action may legitimately accept a text or custom payload,
// and the script is the only thing that knows which.
func TestAction_MalformedBodyDoesNotFailTheRequest(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"raw.lua": `
			return {
				status = 200,
				body = (rela.request.body == nil and "nobody" or "parsed") .. ":" .. rela.request.raw,
				content_type = "text/plain",
			}
		`,
	})
	app.Cfg().Actions = map[string]dataentryconfig.Action{
		"raw": {Script: "raw.lua", Request: &dataentryconfig.ActionRequest{Body: true}},
	}

	rec := postRequestAction(t, app, "raw", `not json at all`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "nobody:not json at all" {
		t.Fatalf("body = %q", got)
	}
}

// TestAction_EntityIDStillResolvesWithRequestBlock pins that the entity_id
// channel survives request-scoped execution. It is read from the SAME parsed
// body, through visibility.ScriptReader (BUG-ZWTDH9) — a request: block must
// not become a way to hand a script an entity by another route.
func TestAction_EntityIDStillResolvesWithRequestBlock(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"ent.lua": `
			return {
				status = 200,
				body = (entity == nil) and "none" or entity.id,
				content_type = "text/plain",
			}
		`,
	})
	app.Cfg().Actions = map[string]dataentryconfig.Action{
		"ent": {Script: "ent.lua", Request: &dataentryconfig.ActionRequest{Body: true}},
	}

	// An id nobody can read (it does not exist) leaves `entity` nil and the
	// action still runs — the documented pre-existing behavior.
	rec := postRequestAction(t, app, "ent", `{"entity_id":"GHOST-1"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "none" {
		t.Fatalf("expected an unreadable entity_id to leave entity nil, got %q", rec.Body.String())
	}
}
