package lua

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestIdpSyncAction runs the SHIPPED examples/idp-sync.lua end to end against a
// stubbed operator API, proving the provider-specific glue works with the generic
// crypto.* + http.* primitives: it signs a Pratique-shaped operator request, and
// upserts a person keyed on sub (idempotently).
func TestIdpSyncAction(t *testing.T) {
	const (
		orgID  = "org_acme"
		userID = "usr_founder"
		email  = "founder@acme.test"
		hmacK  = "shared-hmac-key"
	)

	// A stub operator API that VERIFIES the incoming Pratique signature the way the
	// real server does, then returns the member. Verifying here (not just checking
	// presence) proves the Lua-assembled signature is byte-correct.
	var sawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		date := r.Header.Get("X-Pratique-Date")
		sig := r.Header.Get("X-Pratique-Signature")
		if date == "" || sig == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Recompute canonical = METHOD\nPATH\nDATE\nhex(sha256(body)); body is empty.
		bodySum := sha256.Sum256(nil)
		canonical := r.Method + "\n" + r.URL.Path + "\n" + date + "\n" + hex.EncodeToString(bodySum[:])
		mac := hmac.New(sha256.New, []byte(hmacK))
		_, _ = mac.Write([]byte(canonical))
		want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		if sig != want {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"` + userID + `","email":"` + email + `","roles":["founder"],"status":"active"}`))
	}))
	defer srv.Close()

	ws := newMockWorkspaceWith(personMeta())
	script := readExampleScript(t, "idp-sync.lua")
	secrets := map[string]string{
		"idp_operator_url": srv.URL,
		"idp_operator_key": hmacK,
	}
	params := map[string]string{"user_id": userID, "org_id": orgID, "event": "membership.created"}

	// --- first run: creates the person ---
	runIdpSync(t, ws, script, secrets, params)

	people := ws.entitiesOfType("person")
	if len(people) != 1 {
		t.Fatalf("after first run: %d person entities, want 1", len(people))
	}
	if got := people[0].Properties["sub"]; got != userID {
		t.Errorf("person sub = %v, want %s", got, userID)
	}
	if got := people[0].Properties["email"]; got != email {
		t.Errorf("person email = %v, want %s", got, email)
	}
	if sawPath != "/api/v1/orgs/"+orgID+"/members/"+userID {
		t.Errorf("operator path = %q, want the org/member path", sawPath)
	}

	// --- second run: updates, no duplicate ---
	runIdpSync(t, ws, script, secrets, params)
	if n := len(ws.entitiesOfType("person")); n != 1 {
		t.Fatalf("after second run: %d person entities, want 1 (idempotent upsert)", n)
	}
}

// TestIdpSyncAction_SkipsOn404: a 404 from the operator API (membership gone) is
// acknowledged without creating a person.
func TestIdpSyncAction_SkipsOn404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	ws := newMockWorkspaceWith(personMeta())
	script := readExampleScript(t, "idp-sync.lua")
	secrets := map[string]string{"idp_operator_url": srv.URL, "idp_operator_key": "k"}
	params := map[string]string{"user_id": "usr_gone", "org_id": "org_1"}

	runIdpSync(t, ws, script, secrets, params)
	if n := len(ws.entitiesOfType("person")); n != 0 {
		t.Fatalf("a 404 must not provision a person, got %d", n)
	}
}

// runIdpSync executes the action script with the given secrets + params against
// the workspace's store.
func runIdpSync(t *testing.T, ws *mockWorkspace, script string, secrets, params map[string]string) {
	t.Helper()
	var buf strings.Builder
	rt := NewWriter(ws.services("/tmp"), &buf,
		WithSecrets(secrets),
		WithParams(params),
		WithActionMode(),
	)
	defer rt.Close()
	if _, err := rt.RunActionString(script, "idp-sync.lua"); err != nil {
		t.Fatalf("idp-sync run failed: %v (output: %s)", err, buf.String())
	}
}

// readExampleScript loads a script from the repo's examples/ directory. The test
// runs from internal/lua, so examples/ is three levels up.
func readExampleScript(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "examples", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// personMeta is a metamodel with a person type (sub required + email), the shape
// the idp-sync action provisions.
func personMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"person": {
				IDPrefix: "PSN",
				Properties: map[string]metamodel.PropertyDef{
					"sub":   {Type: "string", Required: true},
					"email": {Type: "string"},
				},
			},
		},
	}
}

// entitiesOfType returns the mock workspace's entities of a given type.
func (m *mockWorkspace) entitiesOfType(typ string) []*entity.Entity {
	out := make([]*entity.Entity, 0)
	for e, err := range m.store.ListEntities(context.Background(), store.EntityQuery{Type: typ}) {
		if err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}
