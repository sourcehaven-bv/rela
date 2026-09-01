package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// newHookTestApp builds an App whose config carries the given hooks. The
// metamodel is newTestAppV1's (ticket/feature), so hooks below act on `ticket`.
func newHookTestApp(t *testing.T, hooks map[string]dataentryconfig.Webhook) *App {
	t.Helper()
	app := newActionTestApp(t, map[string]string{})
	app.Cfg().Webhooks = hooks
	return app
}

// postHook drives a delivery through the REAL production router, so route
// registration and reachability are exercised, not just the handler.
func postHook(t *testing.T, app *App, hookID, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/hooks/"+hookID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "127.0.0.1:8080"
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)
	return rec
}

// listTickets returns every stored ticket, for assertions about how many
// entities a sequence of deliveries produced.
func listTickets(t *testing.T, app *App) []*entityPkg.Entity {
	t.Helper()
	var out []*entityPkg.Entity
	for e, err := range app.store.ListEntities(context.Background(), store.EntityQuery{Type: "ticket"}) {
		if err != nil {
			t.Fatalf("list tickets: %v", err)
		}
		out = append(out, e)
	}
	return out
}

// decodeHookResult decodes the JSON body of a successful delivery.
func decodeHookResult(t *testing.T, rec *httptest.ResponseRecorder) webhookResult {
	t.Helper()
	var res webhookResult
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("decode result: %v (body %q)", err, rec.Body.String())
	}
	return res
}

// TestWebhookRoutes_ThreeWorkflows drives each of the ticket's three shapes end
// to end through the production router.
func TestWebhookRoutes_ThreeWorkflows(t *testing.T) {
	t.Run("always create", func(t *testing.T) {
		app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
			"intake": {
				CreateIfMissing: &dataentryconfig.WebhookCreate{
					Type:       "ticket",
					Properties: map[string]string{"title": "{{body.subject}}", "status": "open"},
				},
			},
		})

		// Two deliveries with identical payloads must produce TWO entities:
		// always-create has no match key and must not dedup.
		for i := range 2 {
			rec := postHook(t, app, "intake", `{"subject":"Printer on fire"}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("delivery %d: status %d (%s)", i, rec.Code, rec.Body.String())
			}
			if got := decodeHookResult(t, rec).Action; got != "created" {
				t.Errorf("delivery %d: action = %q, want created", i, got)
			}
		}

		tickets := listTickets(t, app)
		if len(tickets) != 2 {
			t.Fatalf("always-create produced %d entities, want 2", len(tickets))
		}
		if got := tickets[0].Properties["title"]; got != "Printer on fire" {
			t.Errorf("title = %v, want interpolated body value", got)
		}
	})

	t.Run("find or create", func(t *testing.T) {
		app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
			"alert": {
				Find: &dataentryconfig.WebhookFind{Type: "ticket", Match: []string{"title"}},
				CreateIfMissing: &dataentryconfig.WebhookCreate{
					Properties: map[string]string{"title": "{{body.title}}", "status": "open"},
				},
				Then: []dataentryconfig.WebhookStep{{
					AppendSection: &dataentryconfig.WebhookAppendSection{
						Section: "Notifications",
						Content: "- {{body.state}}",
					},
				}},
			},
		})

		first := postHook(t, app, "alert", `{"title":"web01/http","state":"CRITICAL"}`)
		if first.Code != http.StatusOK {
			t.Fatalf("first delivery: status %d (%s)", first.Code, first.Body.String())
		}
		if got := decodeHookResult(t, first).Action; got != "created" {
			t.Fatalf("first delivery action = %q, want created", got)
		}

		second := postHook(t, app, "alert", `{"title":"web01/http","state":"WARNING"}`)
		if second.Code != http.StatusOK {
			t.Fatalf("second delivery: status %d (%s)", second.Code, second.Body.String())
		}
		if got := decodeHookResult(t, second).Action; got != "updated" {
			t.Fatalf("second delivery action = %q, want updated (it must find the first)", got)
		}

		tickets := listTickets(t, app)
		if len(tickets) != 1 {
			t.Fatalf("find-or-create produced %d entities, want 1", len(tickets))
		}
		// Both notifications must have accumulated in the one entity.
		body := tickets[0].Content
		for _, want := range []string{"CRITICAL", "WARNING", "## Notifications"} {
			if !strings.Contains(body, want) {
				t.Errorf("body missing %q; got %q", want, body)
			}
		}
	})

	t.Run("find and update only", func(t *testing.T) {
		app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
			"resolve": {
				Find: &dataentryconfig.WebhookFind{Type: "ticket", Match: []string{"title"}},
				Then: []dataentryconfig.WebhookStep{{
					Set: map[string]string{"status": "{{body.status}}"},
				}},
			},
		})

		// No match yet: a no-op, and explicitly NOT a create.
		miss := postHook(t, app, "resolve", `{"title":"absent","status":"closed"}`)
		if miss.Code != http.StatusOK {
			t.Fatalf("miss: status %d (%s)", miss.Code, miss.Body.String())
		}
		if got := decodeHookResult(t, miss).Action; got != "no_match" {
			t.Errorf("miss action = %q, want no_match", got)
		}
		if n := len(listTickets(t, app)); n != 0 {
			t.Fatalf("find-and-update-only created %d entities, want 0", n)
		}

		// Seed an entity, then the same delivery must update it.
		created, err := app.write.manager.CreateEntity(context.Background(),
			&entityPkg.Entity{Type: "ticket", Properties: map[string]any{
				"title": "seeded", "status": "open",
			}}, entityPkg.CreateOptions{})
		if err != nil {
			t.Fatalf("seed: %v", err)
		}

		hit := postHook(t, app, "resolve", `{"title":"seeded","status":"closed"}`)
		if hit.Code != http.StatusOK {
			t.Fatalf("hit: status %d (%s)", hit.Code, hit.Body.String())
		}
		if got := decodeHookResult(t, hit).Action; got != "updated" {
			t.Errorf("hit action = %q, want updated", got)
		}

		got, err := app.store.GetEntity(context.Background(), created.Entity.ID)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		if got.Properties["status"] != "closed" {
			t.Errorf("status = %v, want closed", got.Properties["status"])
		}
	})
}

// TestWebhookRoutes_ReachableThroughRouter is the BUG-F3ADZO guard, with the
// same oracle as TestWebhook_ReachableThroughRouter: /hooks/ is not an /api/
// path, so an unregistered route falls through to the SPA catch-all and answers
// 200 HTML — which would read as success to a naive status check.
func TestWebhookRoutes_ReachableThroughRouter(t *testing.T) {
	app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
		"intake": {
			CreateIfMissing: &dataentryconfig.WebhookCreate{
				Type:       "ticket",
				Properties: map[string]string{"title": "{{body.subject}}"},
			},
		},
	})

	rec := postHook(t, app, "intake", `{"subject":"routed"}`)

	if body := rec.Body.String(); strings.Contains(body, "<!") || strings.Contains(body, "<html") {
		t.Fatalf("POST /hooks/intake returned an HTML body — the SPA shell answered, "+
			"not the webhook handler (BUG-F3ADZO). Status %d", rec.Code)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if n := len(listTickets(t, app)); n != 1 {
		t.Fatalf("handler did not run: %d entities created, want 1", n)
	}
}

// TestWebhookRoutes_UnconfiguredHookIsNotMounted pins that only configured hook
// ids are routed — an arbitrary /hooks/<id> must not reach the pipeline.
func TestWebhookRoutes_UnconfiguredHookIsNotMounted(t *testing.T) {
	app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
		"intake": {CreateIfMissing: &dataentryconfig.WebhookCreate{
			Type: "ticket", Properties: map[string]string{"title": "x"},
		}},
	})

	rec := postHook(t, app, "not-configured", `{}`)
	if n := len(listTickets(t, app)); n != 0 {
		t.Fatalf("an unconfigured hook wrote %d entities, want 0", n)
	}
	// It falls through to the SPA catch-all, which is fine — the load-bearing
	// assertion is that no pipeline ran.
	_ = rec
}

// TestWebhookRoutes_BodySizeCap pins that an oversized body is REJECTED rather
// than truncated. Truncation is the dangerous failure: a truncated form body
// parses cleanly and would write a quietly-wrong entity.
func TestWebhookRoutes_BodySizeCap(t *testing.T) {
	app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
		"intake": {
			MaxBodyBytes: 128,
			CreateIfMissing: &dataentryconfig.WebhookCreate{
				Type:       "ticket",
				Properties: map[string]string{"title": "{{body.subject}}"},
			},
		},
	})

	oversized := fmt.Sprintf(`{"subject":%q}`, strings.Repeat("A", 512))
	rec := postHook(t, app, "intake", oversized)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
	if n := len(listTickets(t, app)); n != 0 {
		t.Fatalf("an oversized body produced %d entities, want 0 (it must not be truncated and written)", n)
	}

	// A body just under the cap still works, so the limit is a cap and not a
	// blanket rejection.
	ok := postHook(t, app, "intake", `{"subject":"small"}`)
	if ok.Code != http.StatusOK {
		t.Errorf("under-cap delivery: status = %d, want 200 (%s)", ok.Code, ok.Body.String())
	}
}

// TestWebhookRoutes_HeadersAreAllowlisted pins that a header NOT on the
// allowlist is invisible to interpolation. Headers carry cookies, bearer tokens
// and proxy identity assertions; pass-through would let a hook persist one into
// entity content, where it is then served back on every read.
func TestWebhookRoutes_HeadersAreAllowlisted(t *testing.T) {
	app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
		"intake": {
			Headers: []string{"X-Delivery-Id"},
			CreateIfMissing: &dataentryconfig.WebhookCreate{
				Type: "ticket",
				Properties: map[string]string{
					// The allowlisted header resolves; the non-allowlisted ones
					// must each resolve to empty, leaving only the sentinels.
					"title":  "{{header.x-delivery-id}}",
					"status": "[{{header.authorization}}|{{header.cookie}}|{{header.x-secret}}]",
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/hooks/intake", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Delivery-Id", "d-123")
	req.Header.Set("Authorization", "Bearer super-secret-token")
	req.Header.Set("Cookie", "session=abc123")
	req.Header.Set("X-Secret", "not-allowlisted")
	req.Host = "127.0.0.1:8080"
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}

	tickets := listTickets(t, app)
	if len(tickets) != 1 {
		t.Fatalf("got %d entities, want 1", len(tickets))
	}
	e := tickets[0]

	// The allowlisted header resolved.
	if got := e.Properties["title"]; got != "d-123" {
		t.Errorf("allowlisted header did not resolve: title = %v, want d-123", got)
	}
	// Every non-allowlisted header resolved to empty: the sentinels are all
	// that remains between the separators.
	if got := fmt.Sprintf("%v", e.Properties["status"]); got != "[||]" {
		t.Errorf("a non-allowlisted header leaked into the entity: status = %q, want %q", got, "[||]")
	}
	// Belt and braces: no secret value appears anywhere in the stored entity.
	blob := fmt.Sprintf("%v%v", e.Properties, e.Content)
	for _, secret := range []string{"super-secret-token", "session=abc123", "not-allowlisted"} {
		if strings.Contains(blob, secret) {
			t.Errorf("secret %q reached the stored entity: %s", secret, blob)
		}
	}
}

// TestWebhookRoutes_RespondStatus pins the configurable success status.
func TestWebhookRoutes_RespondStatus(t *testing.T) {
	app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
		"intake": {
			Respond: dataentryconfig.WebhookRespond{Status: http.StatusAccepted},
			CreateIfMissing: &dataentryconfig.WebhookCreate{
				Type: "ticket", Properties: map[string]string{"title": "x"},
			},
		},
	})

	rec := postHook(t, app, "intake", `{}`)
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202 (configured respond.status)", rec.Code)
	}
}

// TestWebhookRoutes_Interpolation covers the template vocabulary, including the
// deliberate choice that an unresolved reference becomes EMPTY rather than
// being left literal — a stored `{{body.host}}` is a silent corruption.
func TestWebhookRoutes_Interpolation(t *testing.T) {
	tests := []struct {
		name     string
		template string
		body     string
		query    string
		want     string
	}{
		{name: "body field", template: "{{body.subject}}", body: `{"subject":"hi"}`, want: "hi"},
		{name: "nested body path", template: "{{body.a.b}}", body: `{"a":{"b":"deep"}}`, want: "deep"},
		{
			name: "integral number has no decimal point", template: "{{body.n}}",
			body: `{"n":8080}`, want: "8080",
		},
		{name: "boolean", template: "{{body.ok}}", body: `{"ok":true}`, want: "true"},
		{name: "query parameter", template: "{{query.src}}", body: `{}`, query: "?src=icinga", want: "icinga"},
		{name: "missing field is empty", template: "x{{body.nope}}y", body: `{}`, want: "xy"},
		{name: "unknown namespace is empty", template: "x{{bogus.k}}y", body: `{}`, want: "xy"},
		{name: "literal text passes through", template: "no refs here", body: `{}`, want: "no refs here"},
		{
			name: "mixed literal and refs", template: "{{body.a}}/{{body.b}}",
			body: `{"a":"x","b":"y"}`, want: "x/y",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
				"intake": {CreateIfMissing: &dataentryconfig.WebhookCreate{
					Type: "ticket", Properties: map[string]string{"title": tc.template},
				}},
			})

			req := httptest.NewRequest(http.MethodPost, "/hooks/intake"+tc.query, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Host = "127.0.0.1:8080"
			rec := httptest.NewRecorder()
			app.NewRouter().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
			}
			tickets := listTickets(t, app)
			if len(tickets) != 1 {
				t.Fatalf("got %d entities, want 1", len(tickets))
			}
			if got := fmt.Sprintf("%v", tickets[0].Properties["title"]); got != tc.want {
				t.Errorf("interpolate(%q) = %q, want %q", tc.template, got, tc.want)
			}
		})
	}
}

// TestWebhookRoutes_FormEncodedBody covers the non-JSON content type, which is
// what an HTML form post delivers.
func TestWebhookRoutes_FormEncodedBody(t *testing.T) {
	app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
		"intake": {CreateIfMissing: &dataentryconfig.WebhookCreate{
			Type: "ticket", Properties: map[string]string{"title": "{{body.subject}}"},
		}},
	})

	req := httptest.NewRequest(http.MethodPost, "/hooks/intake", strings.NewReader("subject=from+a+form"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "127.0.0.1:8080"
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	tickets := listTickets(t, app)
	if len(tickets) != 1 || tickets[0].Properties["title"] != "from a form" {
		t.Fatalf("form body not parsed: %+v", tickets)
	}
}

// TestWebhookRoutes_ContentTypeVariants pins the media-type dispatch across the
// shapes real producers send. The parameter and case handling matter because a
// Content-Type is routinely "application/json; charset=utf-8" rather than the
// bare type, and a naive equality check would silently fall through to the JSON
// branch for a form body — parsing "a=1&b=2" as JSON, failing, and 400-ing a
// delivery the producer will never resend.
//
// Repeated keys are pinned deliberately: url.Values.Get returns the FIRST value,
// so "tag=a&tag=b" keeps "a" and drops "b". A flat map[string]any body cannot
// represent both, and a webhook template addresses one value per key, so this is
// the intended shape rather than an oversight — but it is silent, hence a test
// that says so out loud.
func TestWebhookRoutes_ContentTypeVariants(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantTitle   string
	}{
		{"json", "application/json", `{"subject":"plain json"}`, "plain json"},
		{"json with charset", "application/json; charset=utf-8", `{"subject":"json cs"}`, "json cs"},
		{"absent content type defaults to json", "", `{"subject":"no ct"}`, "no ct"},
		{"form", "application/x-www-form-urlencoded", "subject=a+form", "a form"},
		{"form with charset", "application/x-www-form-urlencoded; charset=utf-8", "subject=form+cs", "form cs"},
		{"form uppercase media type", "APPLICATION/X-WWW-FORM-URLENCODED", "subject=upper", "upper"},
		{"form repeated key keeps the first", "application/x-www-form-urlencoded", "subject=first&subject=second", "first"},
		{"form percent-encoded", "application/x-www-form-urlencoded", "subject=%C3%BCber+alles", "über alles"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
				"intake": {CreateIfMissing: &dataentryconfig.WebhookCreate{
					Type: "ticket", Properties: map[string]string{"title": "{{body.subject}}"},
				}},
			})

			req := httptest.NewRequest(http.MethodPost, "/hooks/intake", strings.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			req.Host = "127.0.0.1:8080"
			rec := httptest.NewRecorder()
			app.NewRouter().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
			}
			tickets := listTickets(t, app)
			if len(tickets) != 1 {
				t.Fatalf("got %d tickets, want 1", len(tickets))
			}
			if got := tickets[0].Properties["title"]; got != tc.wantTitle {
				t.Errorf("title = %q, want %q", got, tc.wantTitle)
			}
		})
	}
}

// TestWebhookRoutes_MalformedFormRejected pins that an unparseable FORM body is
// a 400 and writes nothing, mirroring the JSON case below. Without the explicit
// media-type branch this input would reach the JSON decoder instead and fail
// with a misleading error.
func TestWebhookRoutes_MalformedFormRejected(t *testing.T) {
	app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
		"intake": {CreateIfMissing: &dataentryconfig.WebhookCreate{
			Type: "ticket", Properties: map[string]string{"title": "{{body.subject}}"},
		}},
	})

	req := httptest.NewRequest(http.MethodPost, "/hooks/intake", strings.NewReader("subject=%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "127.0.0.1:8080"
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if tickets := listTickets(t, app); len(tickets) != 0 {
		t.Errorf("got %d tickets, want 0 — a malformed body must write nothing", len(tickets))
	}
}

// TestWebhookRoutes_MalformedJSONRejected pins that an unparseable body is a
// 400 and writes nothing, rather than creating an entity from an empty scope.
func TestWebhookRoutes_MalformedJSONRejected(t *testing.T) {
	app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
		"intake": {CreateIfMissing: &dataentryconfig.WebhookCreate{
			Type: "ticket", Properties: map[string]string{"title": "{{body.subject}}"},
		}},
	})

	rec := postHook(t, app, "intake", `{"subject": `)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if n := len(listTickets(t, app)); n != 0 {
		t.Errorf("malformed body produced %d entities, want 0", n)
	}
}

// TestWebhookRoutes_PatchPreservesUnnamedProperties is the CLAUDE.md
// PatchEntity rule applied to this surface: a then: step that sets one property
// must not erase the others. A read-modify-write through UpdateEntity would,
// and would do so silently for any property the hook's principal cannot read.
func TestWebhookRoutes_PatchPreservesUnnamedProperties(t *testing.T) {
	app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
		"resolve": {
			Find: &dataentryconfig.WebhookFind{Type: "ticket", Match: []string{"title"}},
			Then: []dataentryconfig.WebhookStep{{Set: map[string]string{"status": "closed"}}},
		},
	})

	created, err := app.write.manager.CreateEntity(context.Background(),
		&entityPkg.Entity{
			Type:       "ticket",
			Properties: map[string]any{"title": "keepme", "status": "open"},
			Content:    "Original body.",
		}, entityPkg.CreateOptions{})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	if rec := postHook(t, app, "resolve", `{"title":"keepme"}`); rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}

	got, err := app.store.GetEntity(context.Background(), created.Entity.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Properties["status"] != "closed" {
		t.Errorf("status = %v, want closed", got.Properties["status"])
	}
	if got.Properties["title"] != "keepme" {
		t.Errorf("title was erased: %v", got.Properties["title"])
	}
	if got.Content != "Original body." {
		t.Errorf("content was erased: %q", got.Content)
	}
}

// TestWebhookRoutes_AppendSectionCreatesMissingSection pins the settled
// missing-section behavior at the HTTP boundary: create it, do not error.
// Erroring would discard an alert from a producer that does not retry.
func TestWebhookRoutes_AppendSectionCreatesMissingSection(t *testing.T) {
	app := newHookTestApp(t, map[string]dataentryconfig.Webhook{
		"alert": {
			Find: &dataentryconfig.WebhookFind{Type: "ticket", Match: []string{"title"}},
			Then: []dataentryconfig.WebhookStep{{
				AppendSection: &dataentryconfig.WebhookAppendSection{
					Section: "Notifications", Content: "- {{body.state}}",
				},
			}},
		},
	})

	created, err := app.write.manager.CreateEntity(context.Background(),
		&entityPkg.Entity{
			Type:       "ticket",
			Properties: map[string]any{"title": "no-section"},
			Content:    "Just a summary, with no Notifications heading.",
		}, entityPkg.CreateOptions{})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	if rec := postHook(t, app, "alert", `{"title":"no-section","state":"CRITICAL"}`); rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}

	got, err := app.store.GetEntity(context.Background(), created.Entity.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !strings.Contains(got.Content, "## Notifications") {
		t.Errorf("missing section was not created; body = %q", got.Content)
	}
	if !strings.Contains(got.Content, "CRITICAL") {
		t.Errorf("appended line missing; body = %q", got.Content)
	}
	if !strings.Contains(got.Content, "Just a summary") {
		t.Errorf("original body lost; body = %q", got.Content)
	}
}
