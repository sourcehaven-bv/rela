package openapi

import (
	"slices"
	"testing"
)

func TestBatchPath(t *testing.T) {
	spec := New(attachmentTestMeta(), Config{Title: "T", Version: "1"}).Generate()
	item, ok := spec.Paths["/api/v1/_batch"]
	if !ok || item.Post == nil {
		t.Fatal("spec has no POST /api/v1/_batch")
	}
	requireResponses(t, "batch", item.Post, "200", "400", "403", "409", "413", "422", "501")
	body := item.Post.RequestBody.Content["application/json"].Schema
	for _, list := range []string{"entities", "relations"} {
		if body.Properties[list] == nil {
			t.Errorf("batch body has no %s", list)
		}
	}
	rel := body.Properties["relations"].Items
	if !slices.Equal(rel.Required, []string{"from", "type", "to"}) {
		t.Errorf("relation required = %v", rel.Required)
	}
}
