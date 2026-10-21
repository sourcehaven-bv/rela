package dataentry

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
)

// settingsEditModes are the ACL modes the settings:edit gate distinguishes.
// Under a policy a save needs settings:edit, a read-only instance never
// saves, and without acl.yaml everyone may, as before the gate existed.
var settingsEditModes = []struct {
	name    string
	mode    string // "none", "read-only" or "policy"
	holds   bool
	refused bool
}{
	{name: "no acl.yaml", mode: "none"},
	{name: "read-only", mode: "read-only", holds: true, refused: true},
	{name: "policy, lacks settings:edit", mode: "policy", refused: true},
	{name: "policy, holds settings:edit", mode: "policy", holds: true},
}

// newSettingsEditApp builds a test app in the given ACL mode and a request
// context whose read gate holds every permission or none.
func newSettingsEditApp(t *testing.T, mode string, holds bool) (*App, context.Context) {
	t.Helper()
	app := newTestAppV1(t)
	switch mode {
	case "none":
		setTestACL(t, app, acl.NopACL{})
	case "read-only":
		setTestACL(t, app, acl.ReadOnlyACL{})
	case "policy":
		d, err := acl.NewDeclarative(&acl.Policy{}, aclstore.MustNewGraph(app.store), app.store)
		require.NoError(t, err)
		setTestACL(t, app, d)
	default:
		t.Fatalf("unknown mode %q", mode)
	}
	return app, withReadGate(context.Background(), fakeGate{holdsPermission: holds})
}

// TestSaveSettings_Gate pins who may save the create-form defaults. A
// refused save persists nothing.
func TestSaveSettings_Gate(t *testing.T) {
	for _, tc := range settingsEditModes {
		t.Run(tc.name, func(t *testing.T) {
			app, ctx := newSettingsEditApp(t, tc.mode, tc.holds)
			body := strings.NewReader(`{"defaults":{"status":"open"}}`)
			req := httptest.NewRequest(http.MethodPut, "/api/v1/_settings", body).WithContext(ctx)
			rec := httptest.NewRecorder()

			app.appearance.handleAPISettingsCRUD(rec, req)

			saved := app.settings.UserDefaults()
			if tc.refused {
				require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
				require.Contains(t, rec.Body.String(), acl.PermSettingsEdit)
				require.Nil(t, saved, "a refused save must persist nothing")
				return
			}
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.NotNil(t, saved)
			require.Equal(t, "open", saved.Defaults["status"])
		})
	}
}

// TestSavePalette_Gate pins that the palette, shared by every principal like
// the defaults, is saved under the same gate. A refused save persists
// nothing.
func TestSavePalette_Gate(t *testing.T) {
	for _, tc := range settingsEditModes {
		t.Run(tc.name, func(t *testing.T) {
			app, ctx := newSettingsEditApp(t, tc.mode, tc.holds)
			body := strings.NewReader(`{"accent":"#e11d48"}`)
			req := httptest.NewRequest(http.MethodPut, "/api/v1/_palette", body).WithContext(ctx)
			rec := httptest.NewRecorder()

			app.appearance.handleAPIPaletteCRUD(rec, req)

			saved := app.palette.UserPalette()
			if tc.refused {
				require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
				require.Contains(t, rec.Body.String(), acl.PermSettingsEdit)
				require.Nil(t, saved, "a refused save must persist nothing")
				return
			}
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.NotNil(t, saved)
			require.Equal(t, "#e11d48", saved.Accent)
		})
	}
}

// logoUploadRequest builds a multipart logo upload carrying body.
func logoUploadRequest(ctx context.Context, t *testing.T, method, target, field string, body []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, "logo")
	require.NoError(t, err)
	_, err = fw.Write(body)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	req := httptest.NewRequest(method, target, &buf).WithContext(ctx)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// TestThemeLogo_Gate pins that uploading and removing the shared logo need
// settings:edit. A refused upload stores nothing and a refused delete keeps
// the logo.
func TestThemeLogo_Gate(t *testing.T) {
	for _, tc := range settingsEditModes {
		t.Run(tc.name+", upload", func(t *testing.T) {
			app, ctx := newSettingsEditApp(t, tc.mode, tc.holds)
			req := logoUploadRequest(ctx, t, http.MethodPut, "/api/v1/_theme/logo", "logo", makeTinyPNG(t))
			rec := httptest.NewRecorder()

			app.appearance.handleAPIThemeLogo(rec, req)

			_, ext, _ := app.logo.Get()
			if tc.refused {
				require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
				require.Contains(t, rec.Body.String(), acl.PermSettingsEdit)
				require.Empty(t, ext, "a refused upload must store nothing")
				return
			}
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.NotEmpty(t, ext)
		})
		t.Run(tc.name+", delete", func(t *testing.T) {
			app, ctx := newSettingsEditApp(t, tc.mode, tc.holds)
			require.NoError(t, app.logo.Save(context.Background(), makeTinyPNG(t), "png"))
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/_theme/logo", http.NoBody).WithContext(ctx)
			rec := httptest.NewRecorder()

			app.appearance.handleAPIThemeLogo(rec, req)

			_, ext, _ := app.logo.Get()
			if tc.refused {
				require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
				require.Contains(t, rec.Body.String(), acl.PermSettingsEdit)
				require.Equal(t, "png", ext, "a refused delete must keep the logo")
				return
			}
			require.NotEqual(t, http.StatusForbidden, rec.Code, rec.Body.String())
			require.Empty(t, ext)
		})
	}
}

// TestThemeImport_Gate pins that a theme import, which saves the bundled
// logo, needs settings:edit. A refused import is answered before the upload
// is parsed, so even an invalid package gets the 403.
func TestThemeImport_Gate(t *testing.T) {
	for _, tc := range settingsEditModes {
		t.Run(tc.name, func(t *testing.T) {
			app, ctx := newSettingsEditApp(t, tc.mode, tc.holds)
			req := logoUploadRequest(ctx, t, http.MethodPost, "/api/v1/_theme/import", "file", []byte("not a zip"))
			rec := httptest.NewRecorder()

			app.appearance.handleAPIThemeImport(rec, req)

			if tc.refused {
				require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
				require.Contains(t, rec.Body.String(), acl.PermSettingsEdit)
				return
			}
			// Past the gate the invalid package is rejected on its content.
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}
