package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestOpenAPI_EveryOperationReachesAHandler pins that the served spec only
// describes routes a generic client can call (TKT-3DLP0K). restish turns
// every operation into a command, so a path the router does not serve, or one
// the same-origin check refuses to a non-browser client, is a command that
// always fails.
//
// Each operation runs on a fresh app (some delete the fixture entity) with
// the same-origin layer on, a ticket and an attached file, and a request
// shaped like restish's: no Origin, no cookie. Oracle: not the stdlib 404 (no
// mux route), not the v1 dispatcher's "Resource not found" (no dynamic
// route), not 405, and not 403 (refused by the same-origin check; the fixture
// ACL allows everything).
func TestOpenAPI_EveryOperationReachesAHandler(t *testing.T) {
	const host = "127.0.0.1:8080" // the bind newTestSecurity allows
	probeApp := func(t *testing.T) (*App, http.Handler) {
		t.Helper()
		app := newTestAppV1(t)
		app.security = newTestSecurity(t)
		seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
		router := app.NewRouter()
		req := httptest.NewRequest(http.MethodPut,
			"http://"+host+"/api/v1/tickets/TKT-001/_attachments/screenshot?filename=shot.txt", strings.NewReader("x"))
		req.Header.Set("Content-Type", "application/octet-stream")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("seed attachment: %d %s", rec.Code, rec.Body)
		}
		return app, router
	}

	_, router := probeApp(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://"+host+"/api/v1/_openapi.json", http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET spec: %d %s", rec.Code, rec.Body)
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}

	params := map[string]string{"id": "TKT-001", "property": "screenshot", "fileName": "shot.txt"}
	segment := regexp.MustCompile(`\{([^}]+)\}`)
	methods := map[string]string{
		"get": http.MethodGet, "put": http.MethodPut, "post": http.MethodPost,
		"patch": http.MethodPatch, "delete": http.MethodDelete,
	}
	var probed int
	for path, item := range spec.Paths {
		concrete := segment.ReplaceAllStringFunc(path, func(s string) string {
			if v, ok := params[s[1:len(s)-1]]; ok {
				return v
			}
			return "x"
		})
		for key := range item {
			method, ok := methods[key]
			if !ok {
				continue // "parameters", not an operation
			}
			probed++
			t.Run(method+" "+path, func(t *testing.T) {
				_, router := probeApp(t)
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, httptest.NewRequest(method, "http://"+host+concrete, http.NoBody))
				body := rec.Body.String()
				switch {
				case isStdlibNotFound(rec.Code, body):
					t.Fatalf("%s %s: no route (stdlib 404)", method, concrete)
				case rec.Code == http.StatusMethodNotAllowed:
					t.Fatalf("%s %s: method not allowed", method, concrete)
				case rec.Code == http.StatusNotFound && strings.Contains(body, `"Resource not found"`):
					t.Fatalf("%s %s: no dynamic route", method, concrete)
				case rec.Code == http.StatusForbidden:
					t.Fatalf("%s %s: refused to a non-browser client: %s", method, concrete, body)
				}
			})
		}
	}
	if probed < 10 {
		t.Fatalf("probed only %d operations; spec shape changed?", probed)
	}
}

// TestOpenAPI_EntitySchemaCoversWireFields pins the entity schema to the wire:
// every top-level field a real GET returns must be declared, with the JSON type
// the schema claims. Client generators type their models from the spec, so an
// undeclared or mistyped field is one they drop or fail to decode.
func TestOpenAPI_EntitySchemaCoversWireFields(t *testing.T) {
	app := newTestAppV1(t)
	d := writeACL(t, app)
	app.acl = d
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	if rec := putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "screenshot", "shot.txt", []byte("x")); rec.Code != 200 {
		t.Fatalf("seed attachment: %d %s", rec.Code, rec.Body)
	}

	var spec struct {
		Paths map[string]struct {
			Get *struct {
				Responses map[string]struct {
					Content map[string]struct {
						Schema struct {
							Properties map[string]struct {
								Type string `json:"type"`
								Ref  string `json:"$ref"`
							} `json:"properties"`
						} `json:"schema"`
					} `json:"content"`
				} `json:"responses"`
			} `json:"get"`
		} `json:"paths"`
	}
	data, err := app.State().OpenAPIGen.GenerateJSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	declared := spec.Paths["/api/v1/tickets/{id}"].Get.Responses["200"].Content["application/json"].Schema.Properties
	if len(declared) == 0 {
		t.Fatal("no entity schema for GET /api/v1/tickets/{id}")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	app.handleV1DynamicRoutes(rec, req.WithContext(gateCtxFor(aliceCtx(), t, d)))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET entity: %d %s", rec.Code, rec.Body)
	}
	var wire map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	if _, ok := wire["_attachments"]; !ok {
		t.Fatal("fixture should return _attachments; the test would not cover it")
	}
	jsonType := func(v any) string {
		switch v.(type) {
		case map[string]any:
			return "object"
		case []any:
			return "array"
		case string:
			return "string"
		case bool:
			return "boolean"
		case float64:
			return "number"
		}
		return "null"
	}
	for field, value := range wire {
		decl, ok := declared[field]
		if !ok {
			t.Errorf("wire field %q is not in the spec's entity schema", field)
			continue
		}
		if decl.Type != "" && decl.Type != jsonType(value) {
			t.Errorf("wire field %q is %s, spec says %s", field, jsonType(value), decl.Type)
		}
	}
}

// TestOpenAPI_SpecNamesTheJWTHeader pins the wiring: a router built with a
// JWT gate serves a spec whose security scheme is that gate's header, and a
// non-browser client (no Origin, no cookie) may fetch it.
func TestOpenAPI_SpecNamesTheJWTHeader(t *testing.T) {
	app := newTestAppV1(t)
	if err := app.SetJWTGate(JWTGateConfig{
		Verifier:   gateVerifier{validToken: "good.jwt.token", subject: "usr_alice"},
		HeaderName: "Authorization",
	}); err != nil {
		t.Fatalf("SetJWTGate: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_openapi.json", http.NoBody)
	req.Header.Set("Authorization", "Bearer good.jwt.token")
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET spec without Origin: %d %s", rec.Code, rec.Body)
	}
	var spec struct {
		Components struct {
			SecuritySchemes map[string]struct {
				Type   string `json:"type"`
				Scheme string `json:"scheme"`
			} `json:"securitySchemes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	schemes := spec.Components.SecuritySchemes
	if len(schemes) != 1 {
		t.Fatalf("security schemes = %+v, want one", schemes)
	}
	for _, s := range schemes {
		if s.Type != "http" || s.Scheme != "bearer" {
			t.Errorf("scheme = %+v, want http bearer", s)
		}
	}
}
