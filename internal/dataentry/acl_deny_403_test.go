package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TestHandler_ACLDeny_Returns403Structured restores the AC1.7 contract test
// deleted with internal/dataentry/acl_test.go in #1029 (TKT-RPBFAO, gh#1044):
//
//	When EntityManager returns a *acl.ForbiddenError, the data-entry HTTP
//	handler must respond 403 with a structured JSON body carrying rule_kind,
//	rule_id and reason.
//
// # Why this needs its own test
//
// The 403 *status* turned out to be covered incidentally — mutating
// writeForbiddenIfACLDenied to never fire reddens other tests. The structured
// *body* was covered by nothing: mutating `"error": "forbidden"` to anything
// else left the whole package green. That body is the entire point of AC1.7 —
// an operator debugging a denial needs to know WHICH rule fired, and the
// handler's own godoc cites the AWS IAM lesson that opaque denials are
// unsupportable.
//
// # Why it can be wired now
//
// acl_write_test.go carries a note saying a dataentry-level visible-but-
// write-denied test "would require wiring the test entitymanager with the same
// ACL, which newTestAppV1 does not do". That is no longer true: appbuildtest
// has WithDeclarative, which wires the Declarative as both the write-authz ACL
// and the affordance resolver. So the denial here comes from the REAL
// entitymanager write path, not a stubbed error.
func TestHandler_ACLDeny_Returns403Structured(t *testing.T) {
	t.Parallel()

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:    "Ticket",
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string"},
				},
			},
		},
	}

	// A role that may READ tickets but not UPDATE them: the entity is
	// visible, so the read gate passes and the request reaches the write
	// path — which is the only way to exercise AuthorizeWrite's denial.
	// A role with no read grant would 404 at the gate instead (AC3), and
	// never reach the code under test.
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	if err := fs.MkdirAll(paths.CacheDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	bootstrap := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths))
	if err := bootstrap.Store().CreateEntity(context.Background(),
		&entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"reader": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"bob": "reader"},
	}, bootstrap.Store())

	svc := appbuildtest.New(meta,
		appbuildtest.WithFS(fs, paths),
		appbuildtest.WithStore(bootstrap.Store()),
		appbuildtest.WithDeclarative(d),
	)

	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	app.acl = d
	app.schema.Publish(&Schema{Cfg: &Config{}, Meta: meta})

	bobCtx := principal.With(context.Background(),
		principal.Principal{User: "bob", Tool: principal.ToolDataEntry})
	rec := patchEntityAs(bobCtx, t, app, d, "ticket", "tickets", "TKT-001",
		`{"properties":{"title":"changed"}}`, nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("visible-but-write-denied: got %d, want 403; body=%s", rec.Code, rec.Body)
	}

	// The structured body is the contract. Decoding rather than substring
	// matching so a body that merely CONTAINS the words still fails.
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("403 body is not JSON: %v (body=%s)", err, rec.Body)
	}
	if got["error"] != "forbidden" {
		t.Errorf(`body["error"]: want "forbidden", got %q`, got["error"])
	}
	// rule_kind is asserted by VALUE, not merely non-empty: the whole point of
	// AC1.7 is telling the operator which gate fired, and a mutation emitting
	// a constant like "unknown" would satisfy a non-empty check while saying
	// nothing. "role-grant" is the documented kind for "a role's write list
	// either matched or didn't", which is this scenario.
	if got["rule_kind"] != "role-grant" {
		t.Errorf(`body["rule_kind"]: want "role-grant", got %q (body=%s)`, got["rule_kind"], rec.Body)
	}
	// rule_id and reason stay non-empty assertions rather than exact matches:
	// acl.Decision.Reason is documented as never carrying raw policy data, so
	// pinning its exact wording would invite widening it later to make a test
	// pass — the opposite of what that doc promises.
	for _, key := range []string{"rule_id", "reason"} {
		if got[key] == "" {
			t.Errorf("body[%q] is empty; AC1.7 requires it so an operator can tell which rule fired (body=%s)",
				key, rec.Body)
		}
	}
}
