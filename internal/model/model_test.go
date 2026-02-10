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

// TestEntityGetAttribute tests the GetAttribute method for uniform field/property access
func TestEntityGetAttribute(t *testing.T) {
	e := NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "Test Title"
	e.Properties["priority"] = "high"
	e.Properties["count"] = 42

	tests := []struct {
		name     string
		attrName string
		expected interface{}
	}{
		{"id field", "id", "REQ-001"},
		{"type field", "type", "requirement"},
		{"title property", "title", "Test Title"},
		{"priority property", "priority", "high"},
		{"count property (int)", "count", 42},
		{"missing property", "missing", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.GetAttribute(tt.attrName)
			if got != tt.expected {
				t.Errorf("GetAttribute(%q) = %v, want %v", tt.attrName, got, tt.expected)
			}
		})
	}
}

// TestEntityGetAttributeString tests the GetAttributeString method
func TestEntityGetAttributeString(t *testing.T) {
	e := NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "Test Title"
	e.Properties["count"] = 42
	e.Properties["active"] = true

	tests := []struct {
		name     string
		attrName string
		expected string
	}{
		{"id field", "id", "REQ-001"},
		{"type field", "type", "requirement"},
		{"title property", "title", "Test Title"},
		{"count property (int to string)", "count", "42"},
		{"bool property", "active", "true"},
		{"missing property", "missing", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.GetAttributeString(tt.attrName)
			if got != tt.expected {
				t.Errorf("GetAttributeString(%q) = %q, want %q", tt.attrName, got, tt.expected)
			}
		})
	}
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

// TestParseEntityID_MultiSegmentPrefix tests parsing IDs with multi-segment prefixes
func TestParseEntityID_MultiSegmentPrefix(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantPrefix string
		wantNumber int
		wantRaw    string
	}{
		{
			name:       "single segment prefix",
			input:      "REQ-001",
			wantPrefix: "REQ-",
			wantNumber: 1,
			wantRaw:    "REQ-001",
		},
		{
			name:       "two segment prefix",
			input:      "ISO-CA-001",
			wantPrefix: "ISO-CA-",
			wantNumber: 1,
			wantRaw:    "ISO-CA-001",
		},
		{
			name:       "three segment prefix",
			input:      "ISO-CA-XX-042",
			wantPrefix: "ISO-CA-XX-",
			wantNumber: 42,
			wantRaw:    "ISO-CA-XX-042",
		},
		{
			name:       "two segment no trailing dash",
			input:      "ISO-CA001",
			wantPrefix: "ISO-CA",
			wantNumber: 1,
			wantRaw:    "ISO-CA001",
		},
		{
			name:       "single letter segments",
			input:      "A-B-1",
			wantPrefix: "A-B-",
			wantNumber: 1,
			wantRaw:    "A-B-1",
		},
		{
			name:    "opaque id with no number",
			input:   "PERS-JV",
			wantRaw: "PERS-JV",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ParseEntityID(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertEqual(t, id.Prefix, tt.wantPrefix)
			assertEqual(t, id.Number, tt.wantNumber)
			assertEqual(t, id.Raw, tt.wantRaw)
		})
	}
}

// TestExtractHighestNumber_MultiSegmentPrefix tests number extraction with multi-segment prefixes
func TestExtractHighestNumber_MultiSegmentPrefix(t *testing.T) {
	tests := []struct {
		name   string
		ids    []string
		prefix string
		want   int
	}{
		{
			name:   "single segment",
			ids:    []string{"REQ-001", "REQ-002", "REQ-003"},
			prefix: "REQ-",
			want:   3,
		},
		{
			name:   "multi segment ISO-CA",
			ids:    []string{"ISO-CA-001", "ISO-CA-002", "ISO-CA-010"},
			prefix: "ISO-CA-",
			want:   10,
		},
		{
			name:   "mixed with other prefixes",
			ids:    []string{"ISO-CA-001", "REQ-005", "ISO-CA-003"},
			prefix: "ISO-CA-",
			want:   3,
		},
		{
			name:   "no matches",
			ids:    []string{"REQ-001", "DEC-002"},
			prefix: "ISO-CA-",
			want:   0,
		},
		{
			name:   "empty list",
			ids:    []string{},
			prefix: "ISO-CA-",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractHighestNumber(tt.ids, tt.prefix)
			assertEqual(t, got, tt.want)
		})
	}
}

// TestGenerateNextID_MultiSegmentPrefix tests ID generation with multi-segment prefixes
func TestGenerateNextID_MultiSegmentPrefix(t *testing.T) {
	tests := []struct {
		name   string
		ids    []string
		prefix string
		want   string
	}{
		{
			name:   "first ISO-CA ID",
			ids:    []string{},
			prefix: "ISO-CA-",
			want:   "ISO-CA-001",
		},
		{
			name:   "next ISO-CA ID",
			ids:    []string{"ISO-CA-001"},
			prefix: "ISO-CA-",
			want:   "ISO-CA-002",
		},
		{
			name:   "gap in sequence",
			ids:    []string{"ISO-CA-001", "ISO-CA-005"},
			prefix: "ISO-CA-",
			want:   "ISO-CA-006",
		},
		{
			name:   "single segment still works",
			ids:    []string{"REQ-001", "REQ-002"},
			prefix: "REQ-",
			want:   "REQ-003",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateNextID(tt.ids, tt.prefix)
			assertEqual(t, got, tt.want)
		})
	}
}

// TestEntityIDMatchesPattern_MultiSegment tests pattern matching with multi-segment prefixes
func TestEntityIDMatchesPattern_MultiSegment(t *testing.T) {
	tests := []struct {
		name     string
		id       EntityID
		pattern  string
		expected bool
	}{
		{
			name:     "multi segment exact",
			id:       EntityID{Prefix: "ISO-CA-"},
			pattern:  "ISO-CA-",
			expected: true,
		},
		{
			name:     "multi segment without trailing dash",
			id:       EntityID{Prefix: "ISO-CA-"},
			pattern:  "ISO-CA",
			expected: true,
		},
		{
			name:     "multi segment case insensitive",
			id:       EntityID{Prefix: "ISO-CA-"},
			pattern:  "iso-ca",
			expected: true,
		},
		{
			name:     "multi segment no match",
			id:       EntityID{Prefix: "ISO-CA-"},
			pattern:  "ISO-CB",
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

// TestChangeOpString tests the String method for ChangeOp
func TestChangeOpString(t *testing.T) {
	tests := []struct {
		op   ChangeOp
		want string
	}{
		{OpCreate, "CREATE"},
		{OpModify, "MODIFY"},
		{OpDelete, "DELETE"},
		{OpRename, "RENAME"},
		{ChangeOp(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.op.String(); got != tt.want {
				t.Errorf("ChangeOp.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSyncErrorError tests the Error method for SyncError
func TestSyncErrorError(t *testing.T) {
	err := &SyncError{
		File:    "entities/req/REQ-001.md",
		Message: "invalid YAML frontmatter",
	}
	want := "entities/req/REQ-001.md: invalid YAML frontmatter"
	if got := err.Error(); got != want {
		t.Errorf("SyncError.Error() = %q, want %q", got, want)
	}
}

// TestSortSpecIsDescending tests the IsDescending method for SortSpec
func TestSortSpecIsDescending(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		want      bool
	}{
		{"empty direction", "", false},
		{"asc direction", "asc", false},
		{"desc direction", "desc", true},
		{"other value", "ascending", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := SortSpec{Property: "title", Direction: tt.direction}
			if got := s.IsDescending(); got != tt.want {
				t.Errorf("SortSpec.IsDescending() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGenerateShortID tests short random ID generation
func TestGenerateShortID(t *testing.T) {
	tests := []struct {
		name        string
		existingIDs []string
		prefix      string
		entityCount int
		wantPrefix  string
		wantLength  int
	}{
		{
			name:        "first ID with empty prefix",
			existingIDs: []string{},
			prefix:      "TKT",
			entityCount: 0,
			wantPrefix:  "TKT-",
			wantLength:  4, // 4 chars for small counts
		},
		{
			name:        "prefix with trailing dash",
			existingIDs: []string{},
			prefix:      "REQ-",
			entityCount: 0,
			wantPrefix:  "REQ-",
			wantLength:  4,
		},
		{
			name:        "length scales with entity count 500",
			existingIDs: []string{},
			prefix:      "TKT",
			entityCount: 500,
			wantPrefix:  "TKT-",
			wantLength:  4,
		},
		{
			name:        "length scales with entity count 501",
			existingIDs: []string{},
			prefix:      "TKT",
			entityCount: 501,
			wantPrefix:  "TKT-",
			wantLength:  5,
		},
		{
			name:        "length scales with entity count 1501",
			existingIDs: []string{},
			prefix:      "TKT",
			entityCount: 1501,
			wantPrefix:  "TKT-",
			wantLength:  6,
		},
		{
			name:        "length scales with entity count 10001",
			existingIDs: []string{},
			prefix:      "TKT",
			entityCount: 10001,
			wantPrefix:  "TKT-",
			wantLength:  7,
		},
		{
			name:        "length scales with entity count 50001",
			existingIDs: []string{},
			prefix:      "TKT",
			entityCount: 50001,
			wantPrefix:  "TKT-",
			wantLength:  8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateShortID(tt.existingIDs, tt.prefix, tt.entityCount)

			// Check prefix
			if len(got) < len(tt.wantPrefix) || got[:len(tt.wantPrefix)] != tt.wantPrefix {
				t.Errorf("GenerateShortID() = %q, want prefix %q", got, tt.wantPrefix)
			}

			// Check length (prefix + random part)
			wantTotalLen := len(tt.wantPrefix) + tt.wantLength
			if len(got) != wantTotalLen {
				t.Errorf("GenerateShortID() length = %d, want %d (got %q)", len(got), wantTotalLen, got)
			}

			// Check random part contains only valid base36 characters
			randomPart := got[len(tt.wantPrefix):]
			for _, c := range randomPart {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z')) {
					t.Errorf("GenerateShortID() random part contains invalid char %q in %q", c, got)
				}
			}
		})
	}
}

// TestGenerateShortID_Uniqueness tests that generated IDs are unique
func TestGenerateShortID_Uniqueness(t *testing.T) {
	generated := make(map[string]bool)
	existingIDs := []string{}

	// Generate many IDs and check for uniqueness
	for i := 0; i < 1000; i++ {
		id := GenerateShortID(existingIDs, "TKT", i)
		if generated[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		generated[id] = true
		existingIDs = append(existingIDs, id)
	}
}

// TestGenerateShortID_CollisionAvoidance tests collision handling
func TestGenerateShortID_CollisionAvoidance(t *testing.T) {
	// Pre-populate with some IDs that might collide
	existingIDs := []string{"TKT-0000", "TKT-1111", "TKT-aaaa"}

	// Generate IDs and ensure none collide with existing
	for i := 0; i < 100; i++ {
		id := GenerateShortID(existingIDs, "TKT", len(existingIDs))
		for _, existing := range existingIDs {
			if id == existing {
				t.Errorf("Generated ID %s collides with existing ID", id)
			}
		}
		existingIDs = append(existingIDs, id)
	}
}

// TestCalculateIDLength tests the ID length calculation
func TestCalculateIDLength(t *testing.T) {
	tests := []struct {
		entityCount int
		wantLength  int
	}{
		{0, 4},
		{500, 4},
		{501, 5},
		{1500, 5},
		{1501, 6},
		{10000, 6},
		{10001, 7},
		{50000, 7},
		{50001, 8},
		{100000, 8},
	}

	for _, tt := range tests {
		got := calculateIDLength(tt.entityCount)
		if got != tt.wantLength {
			t.Errorf("calculateIDLength(%d) = %d, want %d", tt.entityCount, got, tt.wantLength)
		}
	}
}
