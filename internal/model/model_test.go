package model

import (
	"testing"
)

// TestNewEntity tests entity creation
func TestNewEntity(t *testing.T) {
	e := NewEntity("REQ-001", "requirement")
	assertEqual(t, e.ID, "REQ-001")
	assertEqual(t, e.Type, "requirement")
	if e.Properties == nil {
		t.Error("expected Properties to be initialized")
	}
}

// Test helpers to avoid import cycle
func assertEqual(t *testing.T, got, want interface{}) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestEntityGetString tests retrieving string properties
func TestEntityGetString(t *testing.T) {
	e := NewEntity("REQ-001", "requirement")

	// Test missing property
	assertEqual(t, e.GetString("missing"), "")

	// Test existing property
	e.Properties["title"] = "Test Title"
	assertEqual(t, e.GetString("title"), "Test Title")

	// Test non-string property
	e.Properties["number"] = 42
	assertEqual(t, e.GetString("number"), "")
}

// TestEntitySetString tests setting string properties
func TestEntitySetString(t *testing.T) {
	e := NewEntity("REQ-001", "requirement")

	e.SetString("title", "Test Title")
	assertEqual(t, e.GetString("title"), "Test Title")

	// Test setting on nil Properties
	e2 := &Entity{}
	e2.SetString("title", "Test")
	if e2.Properties == nil {
		t.Error("expected Properties to be initialized")
	}
	assertEqual(t, e2.GetString("title"), "Test")
}

// TestEntityTitle tests the Title helper method
func TestEntityTitle(t *testing.T) {
	e := NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "My Title"

	assertEqual(t, e.Title(), "My Title")
}

// TestEntityStatus tests the Status helper method
func TestEntityStatus(t *testing.T) {
	e := NewEntity("REQ-001", "requirement")
	e.Properties["status"] = "accepted"

	assertEqual(t, e.Status(), StatusAccepted)

	// Test empty status
	e2 := NewEntity("REQ-002", "requirement")
	assertEqual(t, e2.Status(), Status(""))
}

// TestEntityDescription tests the Description helper method
func TestEntityDescription(t *testing.T) {
	e := NewEntity("REQ-001", "requirement")
	e.Properties["description"] = "My Description"

	assertEqual(t, e.Description(), "My Description")
}

// TestNewRelation tests relation creation
func TestNewRelation(t *testing.T) {
	r := NewRelation("REQ-001", "implements", "DEC-001")
	assertEqual(t, r.From, "REQ-001")
	assertEqual(t, r.Type, "implements")
	assertEqual(t, r.To, "DEC-001")
}

// TestRelationKey tests relation key generation
func TestRelationKey(t *testing.T) {
	r := NewRelation("REQ-001", "implements", "DEC-001")
	assertEqual(t, r.Key(), "REQ-001--implements--DEC-001")
}

// TestEntityIDString tests String method with various scenarios
func TestEntityIDString(t *testing.T) {
	tests := []struct {
		name     string
		id       EntityID
		expected string
	}{
		{
			name:     "with raw",
			id:       EntityID{Raw: "RAW-ID"},
			expected: "RAW-ID",
		},
		{
			name:     "with prefix and number",
			id:       EntityID{Prefix: "REQ-", Number: 42},
			expected: "REQ-42",
		},
		{
			name:     "number only",
			id:       EntityID{Number: 42},
			expected: "42",
		},
		{
			name:     "empty",
			id:       EntityID{},
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.String(); got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

// TestEntityIDNextID tests NextID generation
func TestEntityIDNextID(t *testing.T) {
	id := EntityID{Prefix: "REQ-", Number: 42}
	next := id.NextID()

	if next.Prefix != "REQ-" {
		t.Errorf("expected prefix REQ-, got %s", next.Prefix)
	}
	if next.Number != 43 {
		t.Errorf("expected number 43, got %d", next.Number)
	}
}

// TestEntityIDMatchesPattern tests pattern matching
func TestEntityIDMatchesPattern(t *testing.T) {
	tests := []struct {
		name     string
		id       EntityID
		pattern  string
		expected bool
	}{
		{
			name:     "exact match with dash",
			id:       EntityID{Prefix: "REQ-"},
			pattern:  "REQ-",
			expected: true,
		},
		{
			name:     "exact match without dash",
			id:       EntityID{Prefix: "REQ-"},
			pattern:  "REQ",
			expected: true,
		},
		{
			name:     "case insensitive match",
			id:       EntityID{Prefix: "REQ-"},
			pattern:  "req",
			expected: true,
		},
		{
			name:     "no match",
			id:       EntityID{Prefix: "REQ-"},
			pattern:  "DEC",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.MatchesPattern(tt.pattern); got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

// TestStatusIsValid tests Status validation
func TestStatusIsValid(t *testing.T) {
	tests := []struct {
		status Status
		valid  bool
	}{
		{StatusDraft, true},
		{StatusProposed, true},
		{StatusAccepted, true},
		{StatusDeprecated, true},
		{StatusRejected, true},
		{StatusRetired, true},
		{Status("invalid"), false},
		{Status(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.valid {
				t.Errorf("expected %v, got %v", tt.valid, got)
			}
		})
	}
}

// TestAllStatuses tests that all statuses are returned
func TestAllStatuses(t *testing.T) {
	statuses := AllStatuses()
	if len(statuses) != 6 {
		t.Errorf("expected 6 statuses, got %d", len(statuses))
	}

	// Verify all are valid
	for _, s := range statuses {
		if !s.IsValid() {
			t.Errorf("AllStatuses returned invalid status: %s", s)
		}
	}
}

// TestPriorityIsValid tests Priority validation
func TestPriorityIsValid(t *testing.T) {
	tests := []struct {
		priority Priority
		valid    bool
	}{
		{PriorityCritical, true},
		{PriorityHigh, true},
		{PriorityMedium, true},
		{PriorityLow, true},
		{Priority("invalid"), false},
		{Priority(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.priority), func(t *testing.T) {
			if got := tt.priority.IsValid(); got != tt.valid {
				t.Errorf("expected %v, got %v", tt.valid, got)
			}
		})
	}
}

// TestAllPriorities tests that all priorities are returned
func TestAllPriorities(t *testing.T) {
	priorities := AllPriorities()
	if len(priorities) != 4 {
		t.Errorf("expected 4 priorities, got %d", len(priorities))
	}

	// Verify all are valid
	for _, p := range priorities {
		if !p.IsValid() {
			t.Errorf("AllPriorities returned invalid priority: %s", p)
		}
	}
}
