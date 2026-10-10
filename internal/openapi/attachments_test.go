package openapi

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func attachmentTestMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"task": {
				Label: "Task",
				Properties: map[string]metamodel.PropertyDef{
					"title":    {Type: "string", Required: true},
					"evidence": {Type: metamodel.PropertyTypeFile},
					"contract": {Type: metamodel.PropertyTypeFile},
				},
			},
			"note": {
				Label:      "Note",
				Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}},
			},
		},
	}
}

// TestAttachmentPaths pins the attachment operations a generic client needs:
// a raw-body upload restish can feed with @file, a binary download, a delete,
// and nothing for a type without file properties.
func TestAttachmentPaths(t *testing.T) {
	spec := New(attachmentTestMeta(), Config{Title: "T", Version: "1"}).Generate()

	upload := spec.Paths["/api/v1/tasks/{id}/_attachments/{property}"]
	if upload.Put == nil {
		t.Fatal("missing PUT upload operation")
	}
	if got := upload.Put.OperationID; got != "putTaskAttachment" {
		t.Errorf("upload operationId = %q", got)
	}
	if got := upload.Parameters[1].Schema.Enum; !reflect.DeepEqual(got, []string{"contract", "evidence"}) {
		t.Errorf("property enum = %v, want the sorted file properties", got)
	}
	raw, ok := upload.Put.RequestBody.Content["application/octet-stream"]
	if !ok || raw.Schema.Format != "binary" {
		t.Errorf("upload must accept a binary octet-stream body, got %+v", upload.Put.RequestBody.Content)
	}
	if _, ok := upload.Put.RequestBody.Content["multipart/form-data"]; !ok {
		t.Error("upload must still accept multipart")
	}
	for _, code := range []string{"200", "400", "403", "404", "409", "413", "415", "422", "503"} {
		if _, ok := upload.Put.Responses[code]; !ok {
			t.Errorf("upload missing response %s", code)
		}
	}
	if _, ok := upload.Put.Responses["413"].Content["application/problem+json"]; !ok {
		t.Error("error responses must be application/problem+json")
	}

	file := spec.Paths["/api/v1/tasks/{id}/_attachments/{property}/{fileName}"]
	if file.Get == nil || file.Get.OperationID != "getTaskAttachment" {
		t.Error("missing getTaskAttachment")
	}
	if file.Delete == nil || file.Delete.OperationID != "deleteTaskAttachment" {
		t.Error("missing deleteTaskAttachment")
	}

	for path := range spec.Paths {
		if strings.HasPrefix(path, "/api/v1/notes/") && strings.Contains(path, "_attachments") {
			t.Errorf("type without file properties got attachment path %s", path)
		}
	}
}

// TestSecurityScheme pins how [Config.AuthHeader] is described: Authorization
// is a bearer scheme (what restish's OAuth support sends), any other header an
// API key, and no header means no security at all.
func TestSecurityScheme(t *testing.T) {
	tests := []struct {
		header string
		want   *SecurityScheme
	}{
		{header: "", want: nil},
		{header: "Authorization", want: &SecurityScheme{Type: "http", Scheme: "bearer"}},
		{header: "authorization", want: &SecurityScheme{Type: "http", Scheme: "bearer"}},
		{header: "X-Pomerium-Jwt-Assertion", want: &SecurityScheme{
			Type: "apiKey", In: "header", Name: "X-Pomerium-Jwt-Assertion",
		}},
	}
	for _, tc := range tests {
		t.Run("header="+tc.header, func(t *testing.T) {
			spec := New(attachmentTestMeta(), Config{AuthHeader: tc.header}).Generate()
			got := spec.Components.SecuritySchemes[securitySchemeName]
			if tc.want == nil {
				if got != nil || spec.Security != nil {
					t.Fatalf("no auth header must emit no security, got %+v / %v", got, spec.Security)
				}
				if _, ok := spec.Paths["/api/v1/tasks"].Get.Responses["401"]; ok {
					t.Error("an ungated server must not declare 401")
				}
				return
			}
			if got == nil {
				t.Fatal("missing security scheme")
			}
			got.Description = ""
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("scheme = %+v, want %+v", got, tc.want)
			}
			if len(spec.Security) != 1 {
				t.Errorf("top-level security = %v, want the one scheme", spec.Security)
			}
			if _, ok := spec.Paths["/api/v1/tasks"].Get.Responses["401"]; !ok {
				t.Error("a gated server must declare 401 on its operations")
			}
		})
	}
}

// TestServersDefaultsToOrigin pins that the spec's server resolves against the
// URL it was fetched from: paths carry /api/v1, so the server is the origin.
func TestServersDefaultsToOrigin(t *testing.T) {
	spec := New(attachmentTestMeta(), Config{}).Generate()
	if len(spec.Servers) != 1 || spec.Servers[0].URL != "/" {
		t.Errorf("servers = %+v, want [{URL: /}]", spec.Servers)
	}
}

// TestSpecIsStructurallySound checks what a client generator trips on: every
// $ref resolves, every templated path segment is a declared path parameter,
// and operationIds are unique.
func TestSpecIsStructurallySound(t *testing.T) {
	spec := New(attachmentTestMeta(), Config{AuthHeader: "Authorization"}).Generate()
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}

	for _, m := range regexp.MustCompile(`"\$ref":"#/components/schemas/([^"]+)"`).FindAllStringSubmatch(string(data), -1) {
		if _, ok := spec.Components.Schemas[m[1]]; !ok {
			t.Errorf("unresolved $ref %s", m[1])
		}
	}

	seen := map[string]string{}
	segment := regexp.MustCompile(`\{([^}]+)\}`)
	for path, item := range spec.Paths {
		for method, op := range map[string]*Operation{
			"get": item.Get, "put": item.Put, "post": item.Post, "patch": item.Patch, "delete": item.Delete,
		} {
			if op == nil {
				continue
			}
			if prev, dup := seen[op.OperationID]; dup {
				t.Errorf("operationId %q on %s %s and %s", op.OperationID, method, path, prev)
			}
			seen[op.OperationID] = method + " " + path
			declared := map[string]bool{}
			for _, p := range append(append([]Parameter{}, item.Parameters...), op.Parameters...) {
				if p.In == "path" {
					declared[p.Name] = true
				}
			}
			for _, m := range segment.FindAllStringSubmatch(path, -1) {
				if !declared[m[1]] {
					t.Errorf("%s %s: path parameter {%s} not declared", method, path, m[1])
				}
			}
		}
	}
}

// TestSetAuthHeader_InvalidatesCachedSpec pins that the header set after the
// first Generate reaches the served spec: the cache key is the metamodel, so
// SetAuthHeader must drop the cached copy itself.
func TestSetAuthHeader_InvalidatesCachedSpec(t *testing.T) {
	g := New(attachmentTestMeta(), Config{})
	if _, err := g.GenerateJSON(); err != nil {
		t.Fatal(err)
	}
	g.SetAuthHeader("Authorization")
	data, err := g.GenerateJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"bearer"`) {
		t.Error("spec served after SetAuthHeader has no bearer scheme")
	}
}
