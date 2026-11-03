package openapi

import (
	"slices"
	"testing"
)

// TestPresencePath pins the presence read on every type and its body.
func TestPresencePath(t *testing.T) {
	spec := New(attachmentTestMeta(), Config{Title: "T", Version: "1"}).Generate()
	for _, plural := range []string{"tasks", "notes"} {
		item, ok := spec.Paths["/api/v1/"+plural+"/{id}/_presence"]
		if !ok || item.Get == nil {
			t.Fatalf("%s: missing GET presence operation", plural)
		}
		if item.Post != nil || item.Put != nil || item.Delete != nil || item.Patch != nil {
			t.Errorf("%s: presence has a write operation", plural)
		}
		requireResponses(t, plural+" presence", item.Get, "200", "404", "405")
		body := item.Get.Responses["200"].Content["application/json"].Schema
		if !slices.Equal(body.Required, []string{"viewers", "more"}) {
			t.Errorf("%s: presence required = %v", plural, body.Required)
		}
		viewer := body.Properties["viewers"].Items
		if viewer == nil || !slices.Equal(viewer.Required, []string{"user", "tool", "since"}) {
			t.Errorf("%s: viewer schema = %+v", plural, viewer)
		}
	}
}
