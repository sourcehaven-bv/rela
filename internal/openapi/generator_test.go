package openapi

import (
	"encoding/json"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestGenerator_Generate(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label:       "Ticket",
				LabelPlural: "Tickets",
				Description: "A work item",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "ticket-status"},
				},
			},
			"feature": {
				Label:       "Feature",
				LabelPlural: "Features",
				Properties: map[string]metamodel.PropertyDef{
					"name": {Type: "string", Required: true},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				Label: "implements",
				From:  []string{"ticket"},
				To:    []string{"feature"},
			},
		},
		Types: map[string]metamodel.CustomType{
			"ticket-status": {
				Values:  []string{"open", "in-progress", "done"},
				Default: "open",
			},
		},
	}

	gen := New(meta, Config{
		Title:   "Test API",
		Version: "1.0.0",
	})

	spec := gen.Generate()

	// Check basic structure
	if spec.OpenAPI != "3.1.0" {
		t.Errorf("OpenAPI version = %q, want %q", spec.OpenAPI, "3.1.0")
	}
	if spec.Info.Title != "Test API" {
		t.Errorf("Info.Title = %q, want %q", spec.Info.Title, "Test API")
	}

	// Check paths exist
	expectedPaths := []string{
		"/api/metamodel",
		"/api/search",
		"/api/v1/tickets",
		"/api/v1/tickets/{id}",
		"/api/v1/tickets/{id}/relations",
		"/api/v1/tickets/{id}/relations/implements",
		"/api/v1/features",
		"/api/v1/features/{id}",
	}
	for _, path := range expectedPaths {
		if _, ok := spec.Paths[path]; !ok {
			t.Errorf("Missing path: %s", path)
		}
	}

	// Check ticket collection has GET and POST
	ticketsPath := spec.Paths["/api/v1/tickets"]
	if ticketsPath.Get == nil {
		t.Error("tickets collection missing GET")
	}
	if ticketsPath.Post == nil {
		t.Error("tickets collection missing POST")
	}

	// Check single ticket has GET, PATCH, DELETE
	ticketPath := spec.Paths["/api/v1/tickets/{id}"]
	if ticketPath.Get == nil {
		t.Error("ticket single missing GET")
	}
	if ticketPath.Patch == nil {
		t.Error("ticket single missing PATCH")
	}
	if ticketPath.Delete == nil {
		t.Error("ticket single missing DELETE")
	}

	// Check relation path exists for ticket but not feature
	if _, ok := spec.Paths["/api/v1/tickets/{id}/relations/implements"]; !ok {
		t.Error("Missing implements relation path for ticket")
	}
	if _, ok := spec.Paths["/api/v1/features/{id}/relations/implements"]; ok {
		t.Error("Feature should not have implements relation path (it's not in 'from')")
	}

	// Check schemas
	if spec.Components == nil || spec.Components.Schemas == nil {
		t.Fatal("Missing components/schemas")
	}
	if _, ok := spec.Components.Schemas["Entity"]; !ok {
		t.Error("Missing Entity schema")
	}
	if _, ok := spec.Components.Schemas["Error"]; !ok {
		t.Error("Missing Error schema")
	}
}

func TestGenerator_GenerateJSON(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string"},
				},
			},
		},
	}

	gen := New(meta, Config{})
	data, err := gen.GenerateJSON()
	if err != nil {
		t.Fatalf("GenerateJSON() error = %v", err)
	}

	// Verify valid JSON
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if spec.OpenAPI != "3.1.0" {
		t.Errorf("OpenAPI = %q, want %q", spec.OpenAPI, "3.1.0")
	}
}

func TestGenerator_Caching(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string"},
				},
			},
		},
	}

	gen := New(meta, Config{})

	// First call generates
	spec1 := gen.Generate()

	// Second call should return cached
	spec2 := gen.Generate()

	// Should be same pointer (cached)
	if spec1 != spec2 {
		t.Error("Expected cached spec to be returned")
	}

	// Update metamodel
	newMeta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]metamodel.PropertyDef{
					"title":       {Type: "string"},
					"description": {Type: "string"}, // Added property
				},
			},
		},
	}
	gen.UpdateMetamodel(newMeta)

	// Should regenerate
	spec3 := gen.Generate()
	if spec3 == spec1 {
		t.Error("Expected new spec after metamodel update")
	}
}

func TestGenerator_PropertyTypes(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"test": {
				Label: "Test",
				Properties: map[string]metamodel.PropertyDef{
					"name":       {Type: "string"},
					"created":    {Type: "date"},
					"count":      {Type: "integer"},
					"active":     {Type: "boolean"},
					"attachment": {Type: "file"},
					"status":     {Type: "status", Values: []string{"a", "b", "c"}},
					"tags":       {Type: "string", List: true},
				},
			},
		},
		Types: map[string]metamodel.CustomType{
			"status": {Values: []string{"draft", "published"}},
		},
	}

	gen := New(meta, Config{})
	spec := gen.Generate()

	// Find the create request schema
	createPath := spec.Paths["/api/v1/tests"]
	if createPath.Post == nil || createPath.Post.RequestBody == nil {
		t.Fatal("Missing POST request body")
	}

	// The request body contains a schema with properties
	media := createPath.Post.RequestBody.Content["application/json"]
	if media.Schema == nil {
		t.Fatal("Missing request schema")
	}

	propsSchema := media.Schema.Properties["properties"]
	if propsSchema == nil {
		t.Fatal("Missing properties in request schema")
	}

	tests := []struct {
		prop     string
		wantType string
	}{
		{"name", "string"},
		{"created", "string"}, // date is string with format
		{"count", "integer"},
		{"active", "boolean"},
		{"attachment", "string"}, // file is URI string
		{"tags", "array"},        // list property
	}

	for _, tt := range tests {
		propSchema := propsSchema.Properties[tt.prop]
		if propSchema == nil {
			t.Errorf("Missing property schema for %q", tt.prop)
			continue
		}
		if propSchema.Type != tt.wantType {
			t.Errorf("Property %q type = %q, want %q", tt.prop, propSchema.Type, tt.wantType)
		}
	}

	// Check status has enum values (inline values override custom type)
	statusSchema := propsSchema.Properties["status"]
	if statusSchema == nil {
		t.Fatal("Missing status property")
	}
	if len(statusSchema.Enum) != 3 || statusSchema.Enum[0] != "a" {
		t.Errorf("Status enum = %v, want [a, b, c]", statusSchema.Enum)
	}

	// Check date has format
	createdSchema := propsSchema.Properties["created"]
	if createdSchema.Format != "date" {
		t.Errorf("Created format = %q, want %q", createdSchema.Format, "date")
	}
}

func TestGenerator_DefaultConfig(t *testing.T) {
	meta := &metamodel.Metamodel{}
	gen := New(meta, Config{})

	spec := gen.Generate()

	if spec.Info.Title != "Rela API" {
		t.Errorf("Default title = %q, want %q", spec.Info.Title, "Rela API")
	}
	if spec.Info.Version != "1.0.0" {
		t.Errorf("Default version = %q, want %q", spec.Info.Version, "1.0.0")
	}
}

func TestGenerator_ServerURL(t *testing.T) {
	meta := &metamodel.Metamodel{}
	gen := New(meta, Config{
		ServerURL: "http://localhost:8080",
	})

	spec := gen.Generate()

	if len(spec.Servers) != 1 {
		t.Fatalf("Expected 1 server, got %d", len(spec.Servers))
	}
	if spec.Servers[0].URL != "http://localhost:8080" {
		t.Errorf("Server URL = %q, want %q", spec.Servers[0].URL, "http://localhost:8080")
	}
}

func TestGenerator_DeterministicOutput(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"z-entity": {Label: "Z", Properties: map[string]metamodel.PropertyDef{"z": {Type: "string"}}},
			"a-entity": {Label: "A", Properties: map[string]metamodel.PropertyDef{"a": {Type: "string"}}},
			"m-entity": {Label: "M", Properties: map[string]metamodel.PropertyDef{"m": {Type: "string"}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"z-rel": {Label: "z", From: []string{"a-entity"}, To: []string{"z-entity"}},
			"a-rel": {Label: "a", From: []string{"a-entity"}, To: []string{"m-entity"}},
		},
		Types: map[string]metamodel.CustomType{
			"z-type": {Values: []string{"z"}},
			"a-type": {Values: []string{"a"}},
		},
	}

	gen := New(meta, Config{})

	// Generate multiple times
	jsons := make([]string, 5)
	for i := range 5 {
		gen.Invalidate()
		data, _ := gen.GenerateJSON()
		jsons[i] = string(data)
	}

	// All should be identical
	for i := 1; i < len(jsons); i++ {
		if jsons[i] != jsons[0] {
			t.Errorf("Non-deterministic output at iteration %d", i)
		}
	}
}
