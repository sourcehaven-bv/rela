package dataentry

import (
	"encoding/json"
	"net/http"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestFaceOrder_WorldAbsentPageUsesDeclarationOrder pins, through the HTTP
// surface, that the App's resolver lists faces in declaration order
// (TKT-7IZHP0 design §3.1). The schema declares `published` before `draft`,
// the reverse of token order. POL-1 has neither face in the default world, so
// the view answers the world-absent page over the first readable face, which
// must be the published one.
func TestFaceOrder_WorldAbsentPageUsesDeclarationOrder(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`
entities:
  policy:
    label: Policy
    id_prefix: POL
    faces:
      published: {}
      draft: {}
      review: {}
    properties:
      title: { type: string }
  feature:
    label: Feature
    id_prefix: FEAT
    properties:
      title: { type: string }
`))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	reviewOnly := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"review"}, Fallback: store.FallbackExclude},
	})
	app, _ := facedAppWith(t, meta, reviewOnly, nil)

	rec := viewRecord(t, app, "/api/v1/_views/policy/POL-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET view = %d, want 200 (%s)", rec.Code, rec.Body)
	}
	var resp v1.ViewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.WorldAbsent {
		t.Fatalf("POL-1 has no review face, so the page must be world-absent; got %s", rec.Body)
	}
	if got := resp.Entry.Properties["title"]; got != "PUBLISHED TEXT" {
		t.Errorf("world-absent entry title = %v, want the first DECLARED face's %q", got, "PUBLISHED TEXT")
	}
}
