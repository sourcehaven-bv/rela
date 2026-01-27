package filter

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestSortByID(t *testing.T) {
	entities := []*model.Entity{
		{ID: "C-001"},
		{ID: "A-001"},
		{ID: "B-001"},
	}

	SortByID(entities, false)

	if entities[0].ID != "A-001" || entities[1].ID != "B-001" || entities[2].ID != "C-001" {
		t.Errorf("SortByID ascending failed: got %s, %s, %s", entities[0].ID, entities[1].ID, entities[2].ID)
	}

	SortByID(entities, true)

	if entities[0].ID != "C-001" || entities[1].ID != "B-001" || entities[2].ID != "A-001" {
		t.Errorf("SortByID descending failed: got %s, %s, %s", entities[0].ID, entities[1].ID, entities[2].ID)
	}
}

func TestSortString(t *testing.T) {
	propDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeString}

	entities := []*model.Entity{
		{ID: "1", Properties: map[string]interface{}{"title": "Charlie"}},
		{ID: "2", Properties: map[string]interface{}{"title": "Alice"}},
		{ID: "3", Properties: map[string]interface{}{"title": "Bob"}},
	}

	Sort(entities, "title", propDef, nil, false)

	want := []string{"Alice", "Bob", "Charlie"}
	for i, e := range entities {
		got := e.Properties["title"].(string)
		if got != want[i] {
			t.Errorf("Sort[%d].title = %s, want %s", i, got, want[i])
		}
	}

	// Test descending
	Sort(entities, "title", propDef, nil, true)

	want = []string{"Charlie", "Bob", "Alice"}
	for i, e := range entities {
		got := e.Properties["title"].(string)
		if got != want[i] {
			t.Errorf("Sort descending [%d].title = %s, want %s", i, got, want[i])
		}
	}
}

func TestSortDate(t *testing.T) {
	propDef := &metamodel.PropertyDef{
		Type:   metamodel.PropertyTypeDate,
		Format: "2006-01-02",
	}

	entities := []*model.Entity{
		{ID: "1", Properties: map[string]interface{}{"date": "2025-03-01"}},
		{ID: "2", Properties: map[string]interface{}{"date": "2025-01-15"}},
		{ID: "3", Properties: map[string]interface{}{"date": "2025-02-01"}},
	}

	Sort(entities, "date", propDef, nil, false)

	want := []string{"2025-01-15", "2025-02-01", "2025-03-01"}
	for i, e := range entities {
		got := e.Properties["date"].(string)
		if got != want[i] {
			t.Errorf("Sort[%d].date = %s, want %s", i, got, want[i])
		}
	}
}

func TestSortInteger(t *testing.T) {
	propDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger}

	entities := []*model.Entity{
		{ID: "1", Properties: map[string]interface{}{"score": 10}},
		{ID: "2", Properties: map[string]interface{}{"score": 5}},
		{ID: "3", Properties: map[string]interface{}{"score": 15}},
	}

	Sort(entities, "score", propDef, nil, false)

	want := []int{5, 10, 15}
	for i, e := range entities {
		got, _ := metamodel.ParseIntegerValue(e.Properties["score"])
		if got != want[i] {
			t.Errorf("Sort[%d].score = %d, want %d", i, got, want[i])
		}
	}

	// Test with string integers
	entities = []*model.Entity{
		{ID: "1", Properties: map[string]interface{}{"score": "10"}},
		{ID: "2", Properties: map[string]interface{}{"score": "5"}},
		{ID: "3", Properties: map[string]interface{}{"score": "15"}},
	}

	Sort(entities, "score", propDef, nil, false)

	for i, e := range entities {
		got, _ := metamodel.ParseIntegerValue(e.Properties["score"])
		if got != want[i] {
			t.Errorf("Sort string int [%d].score = %d, want %d", i, got, want[i])
		}
	}
}

func TestSortBoolean(t *testing.T) {
	propDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeBoolean}

	entities := []*model.Entity{
		{ID: "1", Properties: map[string]interface{}{"active": true}},
		{ID: "2", Properties: map[string]interface{}{"active": false}},
		{ID: "3", Properties: map[string]interface{}{"active": true}},
	}

	Sort(entities, "active", propDef, nil, false)

	// false < true, so false should come first
	want := []bool{false, true, true}
	for i, e := range entities {
		got, _ := metamodel.ParseBooleanValue(e.Properties["active"])
		if got != want[i] {
			t.Errorf("Sort[%d].active = %v, want %v", i, got, want[i])
		}
	}
}

func TestSortNilValues(t *testing.T) {
	propDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeString}

	entities := []*model.Entity{
		{ID: "1", Properties: map[string]interface{}{"title": nil}},
		{ID: "2", Properties: map[string]interface{}{"title": "Alice"}},
		{ID: "3", Properties: map[string]interface{}{}}, // missing property
	}

	Sort(entities, "title", propDef, nil, false)

	// Nil values should go to the end
	if entities[0].Properties["title"] != "Alice" {
		t.Error("Non-nil value should come first")
	}
}

func TestSortStability(t *testing.T) {
	propDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeString}

	// Entities with same title should maintain original order
	entities := []*model.Entity{
		{ID: "A", Properties: map[string]interface{}{"title": "Same"}},
		{ID: "B", Properties: map[string]interface{}{"title": "Same"}},
		{ID: "C", Properties: map[string]interface{}{"title": "Same"}},
	}

	Sort(entities, "title", propDef, nil, false)

	// Order should be preserved
	if entities[0].ID != "A" || entities[1].ID != "B" || entities[2].ID != "C" {
		t.Errorf("Sort should be stable: got %s, %s, %s", entities[0].ID, entities[1].ID, entities[2].ID)
	}
}

func TestSortCustomEnumType(t *testing.T) {
	// Test that custom enum types sort by defined order, not alphabetically
	// Priority values are: critical, high, medium, low
	// Alphabetically this would be: critical, high, low, medium (WRONG)
	// By semantic order: critical, high, medium, low (CORRECT)

	meta := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"priority": {
				Values: []string{"critical", "high", "medium", "low"},
			},
		},
	}

	propDef := &metamodel.PropertyDef{Type: "priority"}

	entities := []*model.Entity{
		{ID: "1", Properties: map[string]interface{}{"priority": "low"}},
		{ID: "2", Properties: map[string]interface{}{"priority": "critical"}},
		{ID: "3", Properties: map[string]interface{}{"priority": "medium"}},
		{ID: "4", Properties: map[string]interface{}{"priority": "high"}},
	}

	Sort(entities, "priority", propDef, meta, false)

	// Expected order by metamodel definition: critical (0), high (1), medium (2), low (3)
	want := []string{"critical", "high", "medium", "low"}
	for i, e := range entities {
		got := e.Properties["priority"].(string)
		if got != want[i] {
			t.Errorf("Sort[%d].priority = %s, want %s", i, got, want[i])
		}
	}

	// Test descending order
	Sort(entities, "priority", propDef, meta, true)

	want = []string{"low", "medium", "high", "critical"}
	for i, e := range entities {
		got := e.Properties["priority"].(string)
		if got != want[i] {
			t.Errorf("Sort descending [%d].priority = %s, want %s", i, got, want[i])
		}
	}
}

func TestSortInlineEnumType(t *testing.T) {
	// Test that inline enum values (defined directly in property) sort by defined order
	propDef := &metamodel.PropertyDef{
		Type:   metamodel.PropertyTypeEnum,
		Values: []string{"open", "in-progress", "blocked", "done"},
	}

	entities := []*model.Entity{
		{ID: "1", Properties: map[string]interface{}{"status": "done"}},
		{ID: "2", Properties: map[string]interface{}{"status": "open"}},
		{ID: "3", Properties: map[string]interface{}{"status": "blocked"}},
		{ID: "4", Properties: map[string]interface{}{"status": "in-progress"}},
	}

	Sort(entities, "status", propDef, nil, false)

	// Expected order by property definition: open (0), in-progress (1), blocked (2), done (3)
	want := []string{"open", "in-progress", "blocked", "done"}
	for i, e := range entities {
		got := e.Properties["status"].(string)
		if got != want[i] {
			t.Errorf("Sort[%d].status = %s, want %s", i, got, want[i])
		}
	}
}

func TestSortEnumWithNilValues(t *testing.T) {
	// Test that nil values sort to the end for enum types
	meta := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"priority": {
				Values: []string{"critical", "high", "medium", "low"},
			},
		},
	}

	propDef := &metamodel.PropertyDef{Type: "priority"}

	entities := []*model.Entity{
		{ID: "1", Properties: map[string]interface{}{"priority": nil}},
		{ID: "2", Properties: map[string]interface{}{"priority": "high"}},
		{ID: "3", Properties: map[string]interface{}{}}, // missing property
		{ID: "4", Properties: map[string]interface{}{"priority": "critical"}},
	}

	Sort(entities, "priority", propDef, meta, false)

	// Non-nil values should come first in semantic order, nil values at end
	if entities[0].Properties["priority"] != "critical" {
		t.Errorf("Expected critical first, got %v", entities[0].Properties["priority"])
	}
	if entities[1].Properties["priority"] != "high" {
		t.Errorf("Expected high second, got %v", entities[1].Properties["priority"])
	}
}

func TestSortEnumUnknownValue(t *testing.T) {
	// Test that unknown enum values fall back to string comparison
	meta := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"priority": {
				Values: []string{"critical", "high", "medium", "low"},
			},
		},
	}

	propDef := &metamodel.PropertyDef{Type: "priority"}

	entities := []*model.Entity{
		{ID: "1", Properties: map[string]interface{}{"priority": "unknown"}},
		{ID: "2", Properties: map[string]interface{}{"priority": "critical"}},
		{ID: "3", Properties: map[string]interface{}{"priority": "also-unknown"}},
	}

	Sort(entities, "priority", propDef, meta, false)

	// Known values should come first (in order), unknown values sorted alphabetically at end
	if entities[0].Properties["priority"] != "critical" {
		t.Errorf("Expected critical first, got %v", entities[0].Properties["priority"])
	}
}
