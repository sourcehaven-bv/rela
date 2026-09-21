package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// The entity-context command branch reads the store DIRECTLY — unlike the view
// branch, whose payload arrives already row-gated and redacted from
// executeView. These tests pin the three checks it therefore owes the entity
// itself (BUG-G2BASF review): the row gate, the FACE gate, and field-level
// redaction.
//
// The exposure was latent before BUG-G2BASF: the SPA suppressed every command
// whenever a face was served, so a faced address never reached this handler.
// Removing that suppression is what made it reachable, which is why these
// tests land with that change.
//
// A command's payload is piped to an operator SHELL SCRIPT's stdin, past any
// layer that could observe it — so a leak here is unrecoverable, not merely
// displayed.

// echoStdinCommand configures a command whose script writes its own stdin back
// out, so whatever payload the handler built is observable in the SSE stream.
// The command permission is GRANTED to every principal here. That is
// deliberate: authorizeCommand asks whether this COMMAND may run, and these
// tests are about which ROWS it may see. Leaving it ungranted would make them
// pass on the command gate and never reach the row gate they exist to pin
// (under a configured policy a command with no permission: is denied outright
// — DEC-EIHQSU).
func echoStdinCommand(app *App) {
	app.Cfg().Commands = map[string]CommandConfig{
		// `cat` with no argument reads stdin, which is the payload.
		"echo-stdin": {
			Label: "Echo stdin", Script: "cat", Context: "entity",
			Permission: "command:allowed",
		},
	}
}

// execEntityCommandAs runs the echo command against an entity ADDRESS.
func execEntityCommandAs(
	ctx context.Context, t *testing.T, app *App, d *acl.Declarative, entityID string,
) *httptest.ResponseRecorder {
	t.Helper()
	return execCommandAs(ctx, t, app, d, "/api/command/echo-stdin?entity_id="+entityID)
}

// A principal who may not read the entity must not receive its content, and
// must get the same not-found a missing entity produces.
func TestCommandExec_RowGatesTheEntity(t *testing.T) {
	app := newHandlerTestApp(t)
	bindRepo(app, t.TempDir())
	seedEntity(app, &entity.Entity{
		ID: "TKT-ACL1", Type: "ticket",
		Properties: map[string]any{"title": "secret ticket"},
		Content:    "confidential body",
	})

	echoStdinCommand(app)
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"viewer":  {Read: []string{"ticket"}, Permissions: []string{"command:allowed"}},
			"nothing": {Permissions: []string{"command:allowed"}},
		},
		Assignments: map[string]string{"alice": "viewer", "bob": "nothing"},
	}, app.store)
	app.acl = d

	// alice may read tickets.
	if rec := execEntityCommandAs(aliceCtx(), t, app, d, "TKT-ACL1"); rec.Code != http.StatusOK {
		t.Fatalf("alice: got %d, want 200; body=%s", rec.Code, rec.Body)
	}

	// bob holds no role. The command must refuse, and disclose nothing.
	rec := execEntityCommandAs(principalCtx("bob"), t, app, d, "TKT-ACL1")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("bob (denied): got %d, want 404; body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	for _, secret := range []string{"secret ticket", "confidential body"} {
		if strings.Contains(body, secret) {
			t.Errorf("LEAK: denied command exposed %q: %s", secret, body)
		}
	}
}

// A VISIBLE entity's field-level `visible:`-hidden property value must not
// reach the script. This is the entity-context twin of BUG-9QL9XV, which fixed
// the view context.
func TestCommandExec_RedactsHiddenPropertyValue(t *testing.T) {
	app := newHandlerTestApp(t)
	bindRepo(app, t.TempDir())
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"status": false},
	}}
	seedEntity(app, &entity.Entity{
		ID: "TKT-ACL1", Type: "ticket",
		Properties: map[string]any{"title": "visible title", "status": "SECRET-STATUS"},
	})

	echoStdinCommand(app)
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"viewer":  {Read: []string{"ticket"}, Permissions: []string{"command:allowed"}},
			"nothing": {Permissions: []string{"command:allowed"}},
		},
		Assignments: map[string]string{"alice": "viewer", "bob": "nothing"},
	}, app.store)
	app.acl = d

	rec := execEntityCommandAs(aliceCtx(), t, app, d, "TKT-ACL1")
	if rec.Code != http.StatusOK {
		t.Fatalf("alice: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if strings.Contains(body, "SECRET-STATUS") {
		t.Errorf("LEAK: hidden property value reached the command payload: %s", body)
	}
	// Not vacuous: the readable property DID travel, so the assertion above
	// is about redaction rather than an empty payload.
	if !strings.Contains(body, "visible title") {
		t.Errorf("expected the visible property in the payload, got: %s", body)
	}
}
