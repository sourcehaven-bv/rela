package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
)

// TestSaveSettings_Gate pins who may save the create-form defaults. One set
// serves every principal, so under a policy it needs settings:edit, a
// read-only instance never saves, and without acl.yaml everyone may, as
// before the gate existed. A refused save persists nothing.
func TestSaveSettings_Gate(t *testing.T) {
	cases := []struct {
		name  string
		mode  string // "none", "read-only" or "policy"
		holds bool
		want  int
	}{
		{name: "no acl.yaml", mode: "none", want: http.StatusOK},
		{name: "read-only", mode: "read-only", holds: true, want: http.StatusForbidden},
		{name: "policy, lacks settings:edit", mode: "policy", want: http.StatusForbidden},
		{name: "policy, holds settings:edit", mode: "policy", holds: true, want: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			switch tc.mode {
			case "none":
				setTestACL(t, app, acl.NopACL{})
			case "read-only":
				setTestACL(t, app, acl.ReadOnlyACL{})
			case "policy":
				d, err := acl.NewDeclarative(&acl.Policy{}, aclstore.MustNewGraph(app.store), app.store)
				require.NoError(t, err)
				setTestACL(t, app, d)
			}
			ctx := withReadGate(context.Background(), fakeGate{holdsPermission: tc.holds})
			body := strings.NewReader(`{"defaults":{"status":"open"}}`)
			req := httptest.NewRequest(http.MethodPut, "/api/v1/_settings", body).WithContext(ctx)
			rec := httptest.NewRecorder()

			app.appearance.handleAPISettingsCRUD(rec, req)

			require.Equal(t, tc.want, rec.Code, rec.Body.String())
			saved := app.settings.UserDefaults()
			if tc.want == http.StatusOK {
				require.NotNil(t, saved)
				require.Equal(t, "open", saved.Defaults["status"])
				return
			}
			require.Nil(t, saved, "a refused save must persist nothing")
			require.Contains(t, rec.Body.String(), acl.PermSettingsEdit)
		})
	}
}
