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

	Sort(entities, "title", propDef, false)

	want := []string{"Alice", "Bob", "Charlie"}
	for i, e := range entities {
		got := e.Properties["title"].(string)
		if got != want[i] {
			t.Errorf("Sort[%d].title = %s, want %s", i, got, want[i])
		}
	}

	// Test descending
	Sort(entities, "title", propDef, true)

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

	Sort(entities, "date", propDef, false)

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

	Sort(entities, "score", propDef, false)

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

	Sort(entities, "score", propDef, false)

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

	Sort(entities, "active", propDef, false)

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

	Sort(entities, "title", propDef, false)

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

	Sort(entities, "title", propDef, false)

	// Order should be preserved
	if entities[0].ID != "A" || entities[1].ID != "B" || entities[2].ID != "C" {
		t.Errorf("Sort should be stable: got %s, %s, %s", entities[0].ID, entities[1].ID, entities[2].ID)
	}
}
