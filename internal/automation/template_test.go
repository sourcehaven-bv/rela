package automation

import (
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

func TestInterpolate_SimpleVariables(t *testing.T) {
	t.Parallel()
	vars := TemplateVars{
		Now: func() time.Time { return time.Date(2025, 3, 15, 14, 30, 0, 0, time.UTC) },
		User: UserVars{
			Name:  "John Doe",
			Email: "john@example.com",
		},
	}

	tests := []struct {
		template string
		expected string
	}{
		{"{{today}}", "2025-03-15"},
		{"{{user.name}}", "John Doe"},
		{"{{user.email}}", "john@example.com"},
		{"Created by {{user.name}}", "Created by John Doe"},
		{"No variables here", "No variables here"},
		{"", ""},
	}

	for _, tc := range tests {
		result := Interpolate(tc.template, vars, nil, nil)
		if result != tc.expected {
			t.Errorf("Interpolate(%q) = %q, want %q", tc.template, result, tc.expected)
		}
	}
}

func TestInterpolate_EntityVariables(t *testing.T) {
	t.Parallel()
	vars := DefaultTemplateVars()
	vars.Now = func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) }

	entity := buildEntity(testutil.NewEntity("T-123", "ticket").
		With("status", "in-progress").
		With("owner", "alice"))

	tests := []struct {
		template string
		expected string
	}{
		{"{{entity.id}}", "T-123"},
		{"{{entity.type}}", "ticket"},
		{"{{new.status}}", "in-progress"},
		{"{{new.owner}}", "alice"},
		{"{{new.missing}}", ""},
	}

	for _, tc := range tests {
		result := Interpolate(tc.template, vars, entity, nil)
		if result != tc.expected {
			t.Errorf("Interpolate(%q) = %q, want %q", tc.template, result, tc.expected)
		}
	}
}

func TestInterpolate_OldEntityVariables(t *testing.T) {
	t.Parallel()
	vars := DefaultTemplateVars()
	vars.Now = func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) }

	oldEntity := buildEntity(testutil.NewEntity("T-123", "ticket").
		With("status", "backlog").
		With("owner", "bob"))

	newEntity := buildEntity(testutil.NewEntity("T-123", "ticket").
		With("status", "in-progress").
		With("owner", "alice"))

	tests := []struct {
		template string
		expected string
	}{
		{"{{old.status}}", "backlog"},
		{"{{new.status}}", "in-progress"},
		{"Changed from {{old.owner}} to {{new.owner}}", "Changed from bob to alice"},
	}

	for _, tc := range tests {
		result := Interpolate(tc.template, vars, newEntity, oldEntity)
		if result != tc.expected {
			t.Errorf("Interpolate(%q) = %q, want %q", tc.template, result, tc.expected)
		}
	}
}

func TestInterpolate_NowFormat(t *testing.T) {
	t.Parallel()
	vars := TemplateVars{
		Now: func() time.Time { return time.Date(2025, 6, 15, 14, 30, 45, 0, time.UTC) },
	}

	result := Interpolate("{{now}}", vars, nil, nil)
	// RFC3339 format
	if result != "2025-06-15T14:30:45Z" {
		t.Errorf("Interpolate({{now}}) = %q, want RFC3339 format", result)
	}
}

func TestInterpolate_MixedTemplate(t *testing.T) {
	t.Parallel()
	vars := TemplateVars{
		Now: func() time.Time { return time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC) },
		User: UserVars{
			Name: "Alice",
		},
	}

	entity := buildEntity(testutil.NewEntity("T-001", "ticket").With("title", "Fix bug"))

	template := "{{entity.id}}: {{new.title}} - assigned to {{user.name}} on {{today}}"
	expected := "T-001: Fix bug - assigned to Alice on 2025-01-15"

	result := Interpolate(template, vars, entity, nil)
	if result != expected {
		t.Errorf("Interpolate(%q) = %q, want %q", template, result, expected)
	}
}

func TestInterpolateSafeOnly_SafeVariables(t *testing.T) {
	t.Parallel()
	vars := TemplateVars{
		Now: func() time.Time { return time.Date(2025, 3, 15, 14, 30, 0, 0, time.UTC) },
		User: UserVars{
			Name:  "John Doe",
			Email: "john@example.com",
		},
	}

	tests := []struct {
		template string
		expected string
	}{
		{"{{today}}", "2025-03-15"},
		{"{{user.name}}", "John Doe"},
		{"{{user.email}}", "john@example.com"},
		{"Created by {{user.name}}", "Created by John Doe"},
		{"No variables here", "No variables here"},
		{"", ""},
	}

	for _, tc := range tests {
		result := InterpolateSafeOnly(tc.template, vars)
		if result != tc.expected {
			t.Errorf("InterpolateSafeOnly(%q) = %q, want %q", tc.template, result, tc.expected)
		}
	}
}

func TestInterpolateSafeOnly_DoesNotInterpolateEntityProperties(t *testing.T) {
	t.Parallel()
	// This is the key security test: entity properties should NOT be interpolated
	vars := TemplateVars{
		Now: func() time.Time { return time.Date(2025, 3, 15, 14, 30, 0, 0, time.UTC) },
		User: UserVars{
			Name: "Alice",
		},
	}

	tests := []struct {
		template string
		expected string
	}{
		// These should NOT be interpolated (left as-is)
		{"{{entity.id}}", "{{entity.id}}"},
		{"{{entity.type}}", "{{entity.type}}"},
		{"{{new.title}}", "{{new.title}}"},
		{"{{new.status}}", "{{new.status}}"},
		{"{{old.status}}", "{{old.status}}"},
		// Mixed: safe vars interpolated, entity vars left as-is
		{"{{today}} - {{new.title}}", "2025-03-15 - {{new.title}}"},
		{"{{user.name}}: {{entity.id}}", "Alice: {{entity.id}}"},
	}

	for _, tc := range tests {
		result := InterpolateSafeOnly(tc.template, vars)
		if result != tc.expected {
			t.Errorf("InterpolateSafeOnly(%q) = %q, want %q", tc.template, result, tc.expected)
		}
	}
}

func TestInterpolateSafeOnly_SecurityInjectionPrevention(t *testing.T) {
	t.Parallel()
	// Test that malicious entity properties cannot be injected into Lua code
	vars := TemplateVars{
		Now: func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
	}

	// Imagine an entity with a malicious title
	// If interpolated, this could execute arbitrary code
	maliciousTemplate := `local title = "{{new.title}}" -- {{today}}`

	result := InterpolateSafeOnly(maliciousTemplate, vars)

	// {{new.title}} should NOT be interpolated, only {{today}}
	expected := `local title = "{{new.title}}" -- 2025-01-01`

	if result != expected {
		t.Errorf("InterpolateSafeOnly failed to prevent injection:\ngot: %q\nwant: %q", result, expected)
	}
}

func TestInterpolateSafeOnly_NowFormat(t *testing.T) {
	t.Parallel()
	vars := TemplateVars{
		Now: func() time.Time { return time.Date(2025, 6, 15, 14, 30, 45, 0, time.UTC) },
	}

	result := InterpolateSafeOnly("{{now}}", vars)
	// RFC3339 format
	if result != "2025-06-15T14:30:45Z" {
		t.Errorf("InterpolateSafeOnly({{now}}) = %q, want RFC3339 format", result)
	}
}
