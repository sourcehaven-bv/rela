package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// AC1.7: when EntityManager returns *acl.ForbiddenError, the data-entry
// HTTP handler must respond with HTTP 403 and a structured JSON body
// carrying rule_kind / rule_id / reason.
func TestHandler_ACLDeny_Returns403Structured(t *testing.T) {
	meta := testMeta()
	cfg := testConfig()
	f := &fixture{}

	app := newAppFromParts(cfg, meta, f)
	// Replace the App's services with a ReadOnlyACL-backed bundle.
	fs := storage.NewMemFS()
	ctx := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	svc := appbuildtest.New(meta,
		appbuildtest.WithFS(fs, ctx),
		appbuildtest.WithACL(acl.ReadOnlyACL{}),
	)
	rebindApp(app, fs, ctx, svc)

	// POST /api/entities — minimal valid body, should be denied by ACL
	// before any persistence runs.
	body, _ := json.Marshal(APICreateEntityRequest{
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "Should be denied"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/entities", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.Background())
	rec := httptest.NewRecorder()

	app.handleAPICreateEntity(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d. body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body is not JSON: %v. body = %s", err, rec.Body.String())
	}
	if resp["error"] != "forbidden" {
		t.Errorf(`body["error"] = %q, want "forbidden"`, resp["error"])
	}
	if resp["rule_kind"] != "read-only" {
		t.Errorf(`body["rule_kind"] = %q, want "read-only"`, resp["rule_kind"])
	}
	if resp["rule_id"] != "read-only-acl" {
		t.Errorf(`body["rule_id"] = %q, want "read-only-acl"`, resp["rule_id"])
	}
	if resp["reason"] == "" {
		t.Error(`body["reason"] is empty; want non-empty operator-facing message`)
	}
}
