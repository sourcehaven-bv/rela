package openapi

import "testing"

func TestReconcilePath(t *testing.T) {
	spec := New(attachmentTestMeta(), Config{Title: "T", Version: "1"}).Generate()
	item, ok := spec.Paths["/api/v1/_reconcile"]
	if !ok || item.Post == nil {
		t.Fatal("spec has no POST /api/v1/_reconcile")
	}
	requireResponses(t, "reconcile", item.Post, "200", "400", "401", "413", "422", "429", "501", "503")
	rng := item.Post.RequestBody.Content["application/json"].Schema.Properties["ranges"].Items
	for _, f := range []string{"kind", "entities", "relations", "lo", "after", "hi", "op", "limit"} {
		if rng.Properties[f] == nil {
			t.Errorf("range has no %s", f)
		}
	}
	if h := item.Post.Responses["429"].Headers["Retry-After"]; h.Schema == nil {
		t.Error("429 has no Retry-After header")
	}
}
