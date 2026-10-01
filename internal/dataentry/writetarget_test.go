package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A PATCH on a bare id edits the one face the default world admits and the
// caller may read; with two candidates it is a 422 face_required naming
// them (TKT-7IZHP0 §6). It never edits whichever face the world ranks first.
func TestPatch_BareIDWriteTarget(t *testing.T) {
	ranked := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"published", "draft"}, Fallback: store.FallbackExclude},
	})
	for _, tc := range []struct {
		name      string
		world     store.WorldScope
		wantCode  int
		wantFaces []string
	}{
		{"one candidate", policyPublishedScope(), http.StatusOK, nil},
		{"two candidates", ranked, http.StatusUnprocessableEntity, []string{"POL-1@draft", "POL-1@published"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, _ := facedAppWith(t, facedMeta(t), tc.world, nil)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/policys/POL-1",
				strings.NewReader(`{"properties":{"title":"EDITED"}}`))
			req.Header.Set("Content-Type", "application/json")
			app.NewRouter().ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Fatalf("PATCH = %d %s, want %d", rec.Code, rec.Body, tc.wantCode)
			}
			if tc.wantFaces != nil {
				var problem v1.Error
				if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
					t.Fatal(err)
				}
				if !strings.HasSuffix(problem.Type, "/face_required") || !slices.Equal(problem.Faces, tc.wantFaces) {
					t.Fatalf("problem = %+v, want face_required over %v", problem, tc.wantFaces)
				}
				return
			}
			got, err := app.store.GetEntity(req.Context(), entity.Ref{ID: "POL-1", Face: "published"})
			if err != nil || got.Properties["title"] != "EDITED" {
				t.Fatalf("published face = %v, %v; want the edit there", got, err)
			}
		})
	}
}
