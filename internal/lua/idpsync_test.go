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
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const (
	idpOrgID = "org_acme"
	idpHMAC  = "shared-hmac-key"
)

// idpMembersPath is the operator-API route the action reads. Pratique serves
// GET /api/v1/orgs/{id}/members (the member list) and DELETE on a single
// member, but no GET on a single member (rela#2117).
const idpMembersPath = "/api/v1/orgs/" + idpOrgID + "/members"

// pratiqueStub is an operator API that VERIFIES the Pratique HMAC signature the
// way the real server does and serves only the routes the real server has:
// GET on the member list answers membersJSON, everything else 405 or 404.
// Verifying the signature (not only its presence) proves the Lua-assembled
// signature is byte-correct.
func pratiqueStub(t *testing.T, status int, membersJSON string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+idpMembersPath, func(w http.ResponseWriter, r *http.Request) {
		date := r.Header.Get("X-Pratique-Date")
		sig := r.Header.Get("X-Pratique-Signature")
		bodySum := sha256.Sum256(nil)
		canonical := r.Method + "\n" + r.URL.Path + "\n" + date + "\n" + hex.EncodeToString(bodySum[:])
		mac := hmac.New(sha256.New, []byte(idpHMAC))
		_, _ = mac.Write([]byte(canonical))
		want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		if date == "" || sig != want {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(membersJSON))
	})
	// Pratique's only single-member route. Registering it makes the mux answer
	// 405, as Pratique does, for a GET on a single member.
	mux.HandleFunc("DELETE "+idpMembersPath+"/{mid}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func idpSecrets(url string) map[string]string {
	return map[string]string{"idp_operator_url": url, "idp_operator_key": idpHMAC, "idp_org_id": idpOrgID}
}

// TestIdpSyncAction runs the SHIPPED examples/idp-sync.lua end to end against a
// stubbed operator API, proving the provider-specific glue works with the generic
// crypto.* + http.* primitives: it signs a Pratique-shaped operator request,
// picks the subject out of the member list, and upserts a person keyed on sub
// (idempotently) with the member's roles.
func TestIdpSyncAction(t *testing.T) {
	const (
		userID = "usr_founder"
		email  = "founder@acme.test"
	)
	srv := pratiqueStub(t, http.StatusOK, `{"members":[`+
		`{"user_id":"usr_other","email":"other@acme.test","roles":["admin"],"status":"active"},`+
		`{"user_id":"`+userID+`","email":"`+email+`","roles":["founder"],"status":"active"}]}`)

	ws := newMockWorkspaceWith(personMeta())
	params := map[string]string{"user_id": userID, "org_id": idpOrgID, "event": "membership.created"}

	// --- first run: creates the person ---
	res := runIdpSync(t, ws, idpSecrets(srv.URL), params)
	roles, ok := assertedRoles(t, res)
	if !ok || !slices.Equal(roles, []string{"founder"}) {
		t.Fatalf("asserted_roles = %v (present %v), want [founder]; result %v", roles, ok, res)
	}

	people := ws.people()
	if len(people) != 1 {
		t.Fatalf("after first run: %d person entities, want 1", len(people))
	}
	if got := people[0].Properties["sub"]; got != userID {
		t.Errorf("person sub = %v, want %s", got, userID)
	}
	if got := people[0].Properties["email"]; got != email {
		t.Errorf("person email = %v, want %s", got, email)
	}

	// --- second run: updates, no duplicate ---
	runIdpSync(t, ws, idpSecrets(srv.URL), params)
	if n := len(ws.people()); n != 1 {
		t.Fatalf("after second run: %d person entities, want 1 (idempotent upsert)", n)
	}
}

// TestIdpSyncAction_ClearsRolesUnlessActive pins rela#2117: Pratique offboards
// softly, setting the membership status to "inactive" and leaving its roles in
// place. Only an active member keeps roles; any other status, a missing
// status, or a subject absent from the list clears them, and no person is
// provisioned.
func TestIdpSyncAction_ClearsRolesUnlessActive(t *testing.T) {
	const userID = "usr_leaver"
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"inactive", http.StatusOK,
			`{"members":[{"user_id":"` + userID + `","roles":["founder"],"status":"inactive"}]}`},
		{"pending", http.StatusOK,
			`{"members":[{"user_id":"` + userID + `","roles":["founder"],"status":"pending"}]}`},
		{"status missing", http.StatusOK,
			`{"members":[{"user_id":"` + userID + `","roles":["founder"]}]}`},
		{"not in member list", http.StatusOK,
			`{"members":[{"user_id":"usr_other","roles":["founder"],"status":"active"}]}`},
		{"empty member list (also an unknown org)", http.StatusOK, `{"members":[]}`},
		{"members is an object", http.StatusOK, `{"members":{"user_id":"` + userID + `","status":"active"}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := pratiqueStub(t, tc.status, tc.body)
			ws := newMockWorkspaceWith(personMeta())
			params := map[string]string{"user_id": userID, "org_id": idpOrgID, "event": "membership.updated"}

			res := runIdpSync(t, ws, idpSecrets(srv.URL), params)
			if mt, _ := resultMap(t, res)["message_type"].(string); mt == "error" {
				t.Fatalf("result is an error, want an acknowledged clear: %v", res)
			}
			roles, ok := assertedRoles(t, res)
			if !ok || len(roles) != 0 {
				t.Fatalf("asserted_roles = %v (present %v), want an empty list that clears; result %v", roles, ok, res)
			}
			if n := len(ws.people()); n != 0 {
				t.Fatalf("a non-active subject must not be provisioned, got %d person entities", n)
			}
		})
	}
}

// TestIdpSyncAction_DeactivatesProvisionedMember is the rela#2117 sequence: an
// active member is provisioned with roles, then deactivated. The second run
// clears the roles and leaves the person as it was.
func TestIdpSyncAction_DeactivatesProvisionedMember(t *testing.T) {
	const (
		userID = "usr_leaver"
		email  = "leaver@acme.test"
	)
	member := func(status string) string {
		return `{"members":[{"user_id":"` + userID + `","email":"` + email +
			`","roles":["founder"],"status":"` + status + `"}]}`
	}
	ws := newMockWorkspaceWith(personMeta())
	params := map[string]string{"user_id": userID, "org_id": idpOrgID, "event": "membership.updated"}

	active := pratiqueStub(t, http.StatusOK, member("active"))
	if roles, _ := assertedRoles(t, runIdpSync(t, ws, idpSecrets(active.URL), params)); len(roles) != 1 {
		t.Fatalf("active run asserted %v, want [founder]", roles)
	}
	before := ws.people()

	inactive := pratiqueStub(t, http.StatusOK, member("inactive"))
	res := runIdpSync(t, ws, idpSecrets(inactive.URL), params)
	if roles, ok := assertedRoles(t, res); !ok || len(roles) != 0 {
		t.Fatalf("inactive run asserted %v (present %v), want an empty list; result %v", roles, ok, res)
	}
	after := ws.people()
	if len(before) != 1 || len(after) != 1 {
		t.Fatalf("person count before/after = %d/%d, want 1/1", len(before), len(after))
	}
	if after[0].Properties["email"] != before[0].Properties["email"] {
		t.Errorf("person email changed from %v to %v", before[0].Properties["email"], after[0].Properties["email"])
	}
}

// TestIdpSyncAction_IgnoresOtherOrg: the IdP sends every org's events, and rela
// stores roles per subject. An event for an org other than idp_org_id must
// neither grant that org's roles nor clear this org's.
func TestIdpSyncAction_IgnoresOtherOrg(t *testing.T) {
	srv := pratiqueStub(t, http.StatusOK,
		`{"members":[{"user_id":"usr_1","roles":["owner"],"status":"active"}]}`)
	ws := newMockWorkspaceWith(personMeta())
	params := map[string]string{"user_id": "usr_1", "org_id": "org_elsewhere", "event": "membership.updated"}

	res := runIdpSync(t, ws, idpSecrets(srv.URL), params)
	m := resultMap(t, res)
	if m["message_type"] == "error" {
		t.Fatalf("result is an error, want an acknowledgement: %v", res)
	}
	if _, ok := m["asserted_roles"]; ok {
		t.Fatalf("an event for another org must not carry asserted_roles: %v", res)
	}
	if n := len(ws.people()); n != 0 {
		t.Fatalf("an event for another org must not provision, got %d person entities", n)
	}
}

// TestIdpSyncAction_MissingSecretIsError: rela.secrets leaves out a secret that
// is not configured, so the script sees nil, not "". A missing secret must
// answer an error so the IdP retries, not ignore every event as another org's.
func TestIdpSyncAction_MissingSecretIsError(t *testing.T) {
	for _, missing := range []string{"idp_operator_url", "idp_operator_key", "idp_org_id"} {
		t.Run(missing, func(t *testing.T) {
			srv := pratiqueStub(t, http.StatusOK,
				`{"members":[{"user_id":"usr_1","roles":["owner"],"status":"active"}]}`)
			ws := newMockWorkspaceWith(personMeta())
			secrets := idpSecrets(srv.URL)
			delete(secrets, missing)
			params := map[string]string{"user_id": "usr_1", "org_id": idpOrgID, "event": "membership.updated"}

			m := resultMap(t, runIdpSync(t, ws, secrets, params))
			if m["message_type"] != "error" {
				t.Fatalf("message_type = %v, want error; result %v", m["message_type"], m)
			}
			if _, ok := m["asserted_roles"]; ok {
				t.Fatalf("an error result must not carry asserted_roles: %v", m)
			}
		})
	}
}

// TestIdpSyncAction_DeletedClearsWithoutAPI: membership.deleted proves the
// membership is gone, so a failed fetch still clears the roles instead of
// leaving them until the IdP gives up retrying.
func TestIdpSyncAction_DeletedClearsWithoutAPI(t *testing.T) {
	srv := pratiqueStub(t, http.StatusBadGateway, `{"error":"members lookup failed"}`)
	ws := newMockWorkspaceWith(personMeta())
	params := map[string]string{"user_id": "usr_gone", "org_id": idpOrgID, "event": "membership.deleted"}

	res := runIdpSync(t, ws, idpSecrets(srv.URL), params)
	if mt, _ := resultMap(t, res)["message_type"].(string); mt == "error" {
		t.Fatalf("result is an error, want a clear: %v", res)
	}
	if roles, ok := assertedRoles(t, res); !ok || len(roles) != 0 {
		t.Fatalf("asserted_roles = %v (present %v), want an empty list; result %v", roles, ok, res)
	}
}

// TestIdpSyncAction_ErrorKeepsRoles: a failed operator call answers an error
// result without asserted_roles, so the receiver answers 502, Pratique
// retries, and the stored roles stay as they were. A 404 is an error too:
// Pratique answers 200 with an empty list for an unknown org, so a 404 means
// the request missed the route.
func TestIdpSyncAction_ErrorKeepsRoles(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"server error", http.StatusInternalServerError, `{"error":"boom"}`},
		{"route not found", http.StatusNotFound, "404 page not found"},
		{"bad json", http.StatusOK, `not json`},
		{"no members array", http.StatusOK, `{"error":"unexpected"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := pratiqueStub(t, tc.status, tc.body)
			ws := newMockWorkspaceWith(personMeta())
			params := map[string]string{"user_id": "usr_x", "org_id": idpOrgID, "event": "membership.updated"}

			res := runIdpSync(t, ws, idpSecrets(srv.URL), params)
			m := resultMap(t, res)
			if m["message_type"] != "error" {
				t.Fatalf("message_type = %v, want error; result %v", m["message_type"], res)
			}
			if _, ok := m["asserted_roles"]; ok {
				t.Fatalf("an error result must not carry asserted_roles: %v", res)
			}
		})
	}
}

func resultMap(t *testing.T, res any) map[string]any {
	t.Helper()
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("action result is %T, want a table", res)
	}
	return m
}

// assertedRoles mirrors script.assertedRolesFrom, which this package cannot
// import (script imports lua): an empty Lua table converts to an empty map, a list to []any. The bool
// reports whether the key is present at all.
func assertedRoles(t *testing.T, res any) ([]string, bool) {
	t.Helper()
	v, ok := resultMap(t, res)["asserted_roles"]
	if !ok {
		return nil, false
	}
	switch list := v.(type) {
	case map[string]any:
		if len(list) == 0 {
			return []string{}, true
		}
	case []any:
		out := make([]string, 0, len(list))
		for _, item := range list {
			s, isStr := item.(string)
			if !isStr {
				t.Fatalf("asserted_roles entry %T, want string", item)
			}
			out = append(out, s)
		}
		return out, true
	}
	t.Fatalf("asserted_roles is %T, want a list", v)
	return nil, false
}

// runIdpSync executes the shipped examples/idp-sync.lua with the given secrets +
// params against the workspace's store.
func runIdpSync(t *testing.T, ws *mockWorkspace, secrets, params map[string]string) any {
	t.Helper()
	var buf strings.Builder
	// Grant exactly the secrets this fixture supplies, plus HTTP — the shape a
	// real idp-sync action declares in config (TKT-YH52OM).
	names := make([]string, 0, len(secrets))
	for k := range secrets {
		names = append(names, k)
	}
	rt := NewWriter(ws.services("/tmp"), &buf,
		WithSecrets(secrets),
		WithParams(params),
		WithActionMode(),
		WithCapabilities(Capabilities{HTTP: true, Secrets: names}),
	)
	defer rt.Close()
	res, err := rt.RunActionString(readExampleScript(t, "idp-sync.lua"), "idp-sync.lua")
	if err != nil {
		t.Fatalf("idp-sync run failed: %v (output: %s)", err, buf.String())
	}
	return res
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

// people returns the mock workspace's person entities.
func (m *mockWorkspace) people() []*entity.Entity {
	out := make([]*entity.Entity, 0)
	for e, err := range m.store.ListEntities(context.Background(), store.EntityQuery{Type: "person", Faces: store.InWorld(store.TrivialScope())}) {
		if err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}
