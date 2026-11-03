package openapi

import (
	"slices"
	"testing"
)

// TestLeasePath pins the claim operation on every type, the renew, release
// and list routes, and the `_lease` field on the entity schema.
func TestLeasePath(t *testing.T) {
	spec := New(attachmentTestMeta(), Config{Title: "T", Version: "1"}).Generate()
	for _, plural := range []string{"tasks", "notes"} {
		claim := spec.Paths["/api/v1/"+plural+"/{id}/_lease"]
		if claim.Post == nil {
			t.Fatalf("%s: missing POST claim operation", plural)
		}
		requireResponses(t, plural+" claim", claim.Post, "201", "400", "403", "404", "409", "413", "422", "503")
		held := claim.Post.Responses["409"].Content["application/problem+json"].Schema
		if held == nil || len(held.AllOf) != 2 || held.AllOf[1].Properties["holder"] == nil {
			t.Errorf("%s: the claim 409 does not describe the lease_held holder", plural)
		}
	}

	for _, tc := range []struct {
		path, method string
		codes        []string
		required     []string
	}{
		{"/api/v1/_leases", "GET", []string{"200", "403"}, nil},
		{"/api/v1/_leases/renew", "POST", []string{"200", "400", "403", "409", "413"}, []string{"lease_id"}},
		{"/api/v1/_leases/release", "POST", []string{"204", "400", "403", "413"}, []string{"lease_id"}},
	} {
		item, ok := spec.Paths[tc.path]
		if !ok {
			t.Errorf("spec has no %s", tc.path)
			continue
		}
		op := item.Get
		if tc.method == "POST" {
			op = item.Post
		}
		if op == nil {
			t.Errorf("%s has no %s operation", tc.path, tc.method)
			continue
		}
		requireResponses(t, tc.path, op, tc.codes...)
		if tc.required != nil {
			body := op.RequestBody.Content["application/json"].Schema
			if !slices.Equal(body.Required, tc.required) {
				t.Errorf("%s body required = %v, want %v", tc.path, body.Required, tc.required)
			}
		}
	}

	get := spec.Paths["/api/v1/tasks/{id}"].Get
	if get == nil {
		t.Fatal("missing GET entity operation")
	}
	schema := get.Responses["200"].Content["application/json"].Schema
	l, ok := schema.Properties["_lease"]
	if !ok {
		t.Fatal("entity schema has no _lease")
	}
	if want := []string{"user", "tool", "since"}; !slices.Equal(l.Required, want) {
		t.Errorf("_lease required = %v, want %v", l.Required, want)
	}
}

func requireResponses(t *testing.T, name string, op *Operation, codes ...string) {
	t.Helper()
	for _, code := range codes {
		if _, ok := op.Responses[code]; !ok {
			t.Errorf("%s has no %s response", name, code)
		}
	}
}
