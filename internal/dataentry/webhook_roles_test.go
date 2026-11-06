package dataentry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// recordingRoleStore records the Set calls the webhook receiver makes.
type recordingRoleStore struct {
	calls   []roleSetCall
	err     error
	holdErr error
	held    []string
	holding bool
}

func (s *recordingRoleStore) Hold(_ context.Context, subject string) (func(), error) {
	if s.holdErr != nil {
		return nil, s.holdErr
	}
	s.held = append(s.held, subject)
	s.holding = true
	return func() { s.holding = false }, nil
}

type roleSetCall struct {
	subject, org string
	roles        []string
}

func (s *recordingRoleStore) Set(_ context.Context, subject, orgID string, roles []string) error {
	if !s.holding {
		return errors.New("Set called without holding the subject")
	}
	s.calls = append(s.calls, roleSetCall{subject: subject, org: orgID, roles: slices.Clone(roles)})
	return s.err
}

// TestHandleV1Action_RefusesAssertedRoles: only the webhook receiver stores
// asserted roles, so the HTTP action endpoint fails a response carrying the
// key instead of silently dropping it.
func TestHandleV1Action_RefusesAssertedRoles(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"grant.lua": `return { message = "ok", asserted_roles = { "admin" } }`,
	})
	app.Cfg().Actions = map[string]dataentryconfig.Action{"grant": {Script: "grant.lua"}}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/grant", http.NoBody)
	rec := httptest.NewRecorder()
	callAction(app, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body %q)", rec.Code, rec.Body.String())
	}
}

// TestWebhook_StoresAssertedRoles pins TKT-IZL6NX8: the receiver stores the
// roles the action returned for the subject and org of the VERIFIED claims,
// never for anything the script names.
func TestWebhook_StoresAssertedRoles(t *testing.T) {
	tests := []struct {
		name      string
		script    string
		storeErr  error
		holdErr   error
		noStore   bool
		wantCode  int
		wantCalls []roleSetCall
	}{
		{
			name:     "roles stored for the verified subject",
			script:   `return { message = "ok", asserted_roles = { "cpo", "fg" } }`,
			wantCode: http.StatusOK,
			wantCalls: []roleSetCall{
				{subject: "usr_1", org: "org_1", roles: []string{"cpo", "fg"}},
			},
		},
		{
			name:      "empty list clears",
			script:    `return { asserted_roles = {} }`,
			wantCode:  http.StatusOK,
			wantCalls: []roleSetCall{{subject: "usr_1", org: "org_1", roles: []string{}}},
		},
		{
			name:     "a script cannot name another subject",
			script:   `return { asserted_roles = { "cpo" }, user_id = "usr_admin", subject = "usr_admin" }`,
			wantCode: http.StatusOK,
			wantCalls: []roleSetCall{
				{subject: "usr_1", org: "org_1", roles: []string{"cpo"}},
			},
		},
		{
			name:     "no key leaves roles alone",
			script:   `return { message = "ok" }`,
			wantCode: http.StatusOK,
		},
		{
			name:     "an error response stores nothing and asks for a retry",
			script:   `return { message_type = "error", message = "fetch failed", asserted_roles = { "cpo" } }`,
			wantCode: http.StatusBadGateway,
		},
		{
			name:     "a failing script stores nothing",
			script:   `error("boom")`,
			wantCode: http.StatusBadGateway,
		},
		{
			name:     "a store failure asks for a retry",
			script:   `return { asserted_roles = { "cpo" } }`,
			storeErr: errors.New("disk full"),
			wantCode: http.StatusBadGateway,
			wantCalls: []roleSetCall{
				{subject: "usr_1", org: "org_1", roles: []string{"cpo"}},
			},
		},
		{
			name:     "a hold failure runs nothing",
			script:   `return { asserted_roles = { "cpo" } }`,
			holdErr:  errors.New("lock timeout"),
			wantCode: http.StatusBadGateway,
		},
		{
			name:     "no store wired fails rather than dropping the roles",
			script:   `return { asserted_roles = { "cpo" } }`,
			noStore:  true,
			wantCode: http.StatusBadGateway,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := stubWebhookVerifier{claims: WebhookClaims{
				Event: "membership.updated", UserID: "usr_1", OrgID: "org_1", ID: "evt_" + tc.name,
			}}
			app := newWebhookTestApp(t, tc.script, v)
			store := &recordingRoleStore{err: tc.storeErr, holdErr: tc.holdErr}
			if !tc.noStore {
				app.webhook.roles = store
			}

			rec := postWebhook(app, "jwt")
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tc.wantCode, rec.Body.String())
			}
			if !tc.noStore && tc.holdErr == nil && !slices.Equal(store.held, []string{"usr_1"}) {
				t.Fatalf("held = %v, want the verified subject once", store.held)
			}
			if store.holding {
				t.Fatal("the subject hold was not released")
			}
			if len(store.calls) != len(tc.wantCalls) {
				t.Fatalf("Set calls = %+v, want %+v", store.calls, tc.wantCalls)
			}
			for i, want := range tc.wantCalls {
				got := store.calls[i]
				if got.subject != want.subject || got.org != want.org || !slices.Equal(got.roles, want.roles) {
					t.Fatalf("Set call %d = %+v, want %+v", i, got, want)
				}
			}
		})
	}
}
