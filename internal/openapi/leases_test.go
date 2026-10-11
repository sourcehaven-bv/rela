package openapi

import (
	"strings"
	"testing"
)

// TestLeasePath pins the claim operation on every type, the `_lease` field
// on the entity schema, and that the browser-only /_leases routes stay out
// of the spec.
func TestLeasePath(t *testing.T) {
	spec := New(attachmentTestMeta(), Config{Title: "T", Version: "1"}).Generate()
	for _, plural := range []string{"tasks", "notes"} {
		claim := spec.Paths["/api/v1/"+plural+"/{id}/_lease"]
		if claim.Post == nil {
			t.Fatalf("%s: missing POST claim operation", plural)
		}
		for _, code := range []string{"201", "400", "403", "404", "409", "422", "503"} {
			if _, ok := claim.Post.Responses[code]; !ok {
				t.Errorf("%s: claim has no %s response", plural, code)
			}
		}
	}
	for path := range spec.Paths {
		if strings.HasPrefix(path, "/api/v1/_leases") {
			t.Errorf("spec lists the browser-only route %s", path)
		}
	}
	get := spec.Paths["/api/v1/tasks/{id}"].Get
	if get == nil {
		t.Fatal("missing GET entity operation")
	}
	schema := get.Responses["200"].Content["application/json"].Schema
	if _, ok := schema.Properties["_lease"]; !ok {
		t.Fatal("entity schema has no _lease")
	}
}
