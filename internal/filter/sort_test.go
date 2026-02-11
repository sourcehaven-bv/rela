package filter

import (
	"testing"
	"time"

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

func TestSortDateTimeValues(t *testing.T) {
	// YAML parser produces time.Time values for dates, not strings.
	// This test verifies that compareDates handles time.Time correctly.
	propDef := &metamodel.PropertyDef{
		Type:   metamodel.PropertyTypeDate,
		Format: "2006-01-02",
	}

	d1 := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)

	entities := []*model.Entity{
		{ID: "1", Properties: map[string]interface{}{"date": d1}},
		{ID: "2", Properties: map[string]interface{}{"date": d2}},
		{ID: "3", Properties: map[string]interface{}{"date": d3}},
	}

	Sort(entities, "date", propDef, nil, false)

	wantIDs := []string{"2", "3", "1"}
	for i, e := range entities {
		if e.ID != wantIDs[i] {
			t.Errorf("Sort[%d].ID = %s, want %s", i, e.ID, wantIDs[i])
		}
	}

	// Test with nil date sorts to end
	entities = []*model.Entity{
		{ID: "1", Properties: map[string]interface{}{"date": d1}},
		{ID: "2", Properties: map[string]interface{}{}},
		{ID: "3", Properties: map[string]interface{}{"date": d2}},
	}

	Sort(entities, "date", propDef, nil, false)

	wantIDs = []string{"3", "1", "2"}
	for i, e := range entities {
		if e.ID != wantIDs[i] {
			t.Errorf("Sort with nil[%d].ID = %s, want %s", i, e.ID, wantIDs[i])
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

// --- SortMulti tests ---

func TestSortMulti_SingleSpec(t *testing.T) {
	entityDefs := map[string]*metamodel.EntityDef{
		"item": {Properties: map[string]metamodel.PropertyDef{
			"title": {Type: metamodel.PropertyTypeString},
		}},
	}

	entities := []*model.Entity{
		{ID: "1", Type: "item", Properties: map[string]interface{}{"title": "Charlie"}},
		{ID: "2", Type: "item", Properties: map[string]interface{}{"title": "Alice"}},
		{ID: "3", Type: "item", Properties: map[string]interface{}{"title": "Bob"}},
	}

	SortMulti(entities, []model.SortSpec{{Property: "title"}}, entityDefs, nil)

	want := []string{"Alice", "Bob", "Charlie"}
	for i, e := range entities {
		if e.Properties["title"] != want[i] {
			t.Errorf("SortMulti[%d].title = %v, want %s", i, e.Properties["title"], want[i])
		}
	}
}

func TestSortMulti_MultipleSpecs(t *testing.T) {
	meta := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"priority": {Values: []string{"high", "medium", "low"}},
		},
	}
	entityDefs := map[string]*metamodel.EntityDef{
		"item": {Properties: map[string]metamodel.PropertyDef{
			"priority": {Type: "priority"},
			"title":    {Type: metamodel.PropertyTypeString},
		}},
	}

	entities := []*model.Entity{
		{ID: "1", Type: "item", Properties: map[string]interface{}{"priority": "high", "title": "Zebra"}},
		{ID: "2", Type: "item", Properties: map[string]interface{}{"priority": "high", "title": "Alpha"}},
		{ID: "3", Type: "item", Properties: map[string]interface{}{"priority": "low", "title": "Beta"}},
		{ID: "4", Type: "item", Properties: map[string]interface{}{"priority": "medium", "title": "Gamma"}},
	}

	// Sort by priority asc, then title asc as tiebreaker
	SortMulti(entities, []model.SortSpec{
		{Property: "priority"},
		{Property: "title"},
	}, entityDefs, meta)

	// Expected: high+Alpha, high+Zebra, medium+Gamma, low+Beta
	wantIDs := []string{"2", "1", "4", "3"}
	for i, e := range entities {
		if e.ID != wantIDs[i] {
			t.Errorf("SortMulti multiple [%d].ID = %s, want %s", i, e.ID, wantIDs[i])
		}
	}
}

func TestSortMulti_IDVirtualProperty(t *testing.T) {
	entities := []*model.Entity{
		{ID: "C-001", Type: "item"},
		{ID: "A-001", Type: "item"},
		{ID: "B-001", Type: "item"},
	}

	SortMulti(entities, []model.SortSpec{{Property: "id"}}, nil, nil)

	wantIDs := []string{"A-001", "B-001", "C-001"}
	for i, e := range entities {
		if e.ID != wantIDs[i] {
			t.Errorf("SortMulti id [%d] = %s, want %s", i, e.ID, wantIDs[i])
		}
	}

	// Descending
	SortMulti(entities, []model.SortSpec{{Property: "id", Direction: "desc"}}, nil, nil)

	wantIDs = []string{"C-001", "B-001", "A-001"}
	for i, e := range entities {
		if e.ID != wantIDs[i] {
			t.Errorf("SortMulti id desc [%d] = %s, want %s", i, e.ID, wantIDs[i])
		}
	}
}

func TestSortMulti_ModifiedVirtualProperty(t *testing.T) {
	now := time.Now()
	entities := []*model.Entity{
		{ID: "1", Type: "item", ModTime: now.Add(-2 * time.Hour)},
		{ID: "2", Type: "item", ModTime: now},
		{ID: "3", Type: "item", ModTime: now.Add(-1 * time.Hour)},
	}

	SortMulti(entities, []model.SortSpec{{Property: "modified"}}, nil, nil)

	// Oldest first
	wantIDs := []string{"1", "3", "2"}
	for i, e := range entities {
		if e.ID != wantIDs[i] {
			t.Errorf("SortMulti modified [%d] = %s, want %s", i, e.ID, wantIDs[i])
		}
	}

	// Newest first
	SortMulti(entities, []model.SortSpec{{Property: "modified", Direction: "desc"}}, nil, nil)

	wantIDs = []string{"2", "3", "1"}
	for i, e := range entities {
		if e.ID != wantIDs[i] {
			t.Errorf("SortMulti modified desc [%d] = %s, want %s", i, e.ID, wantIDs[i])
		}
	}
}

func TestSortMulti_ModifiedZeroTimeSortsToEnd(t *testing.T) {
	now := time.Now()
	entities := []*model.Entity{
		{ID: "1", Type: "item"}, // zero time
		{ID: "2", Type: "item", ModTime: now},
		{ID: "3", Type: "item"}, // zero time
	}

	SortMulti(entities, []model.SortSpec{{Property: "modified"}}, nil, nil)

	if entities[0].ID != "2" {
		t.Errorf("Expected entity with ModTime first, got %s", entities[0].ID)
	}
}

func TestSortMulti_MixedEntityTypes(t *testing.T) {
	entityDefs := map[string]*metamodel.EntityDef{
		"requirement": {Properties: map[string]metamodel.PropertyDef{
			"title": {Type: metamodel.PropertyTypeString},
		}},
		"decision": {Properties: map[string]metamodel.PropertyDef{
			"title": {Type: metamodel.PropertyTypeString},
		}},
	}

	entities := []*model.Entity{
		{ID: "1", Type: "requirement", Properties: map[string]interface{}{"title": "Zulu"}},
		{ID: "2", Type: "decision", Properties: map[string]interface{}{"title": "Alpha"}},
		{ID: "3", Type: "requirement", Properties: map[string]interface{}{"title": "Mike"}},
	}

	SortMulti(entities, []model.SortSpec{{Property: "title"}}, entityDefs, nil)

	wantIDs := []string{"2", "3", "1"}
	for i, e := range entities {
		if e.ID != wantIDs[i] {
			t.Errorf("SortMulti mixed types [%d] = %s, want %s", i, e.ID, wantIDs[i])
		}
	}
}

func TestSortMulti_NilPropertyOnSomeEntities(t *testing.T) {
	entityDefs := map[string]*metamodel.EntityDef{
		"item": {Properties: map[string]metamodel.PropertyDef{
			"priority": {Type: metamodel.PropertyTypeInteger},
		}},
	}

	entities := []*model.Entity{
		{ID: "1", Type: "item", Properties: map[string]interface{}{}},
		{ID: "2", Type: "item", Properties: map[string]interface{}{"priority": 5}},
		{ID: "3", Type: "item", Properties: map[string]interface{}{"priority": 1}},
	}

	SortMulti(entities, []model.SortSpec{{Property: "priority"}}, entityDefs, nil)

	// Entities with values first (sorted), nil at end
	if entities[0].ID != "3" {
		t.Errorf("Expected entity 3 (priority 1) first, got %s", entities[0].ID)
	}
	if entities[1].ID != "2" {
		t.Errorf("Expected entity 2 (priority 5) second, got %s", entities[1].ID)
	}
	if entities[2].ID != "1" {
		t.Errorf("Expected entity 1 (no priority) last, got %s", entities[2].ID)
	}
}

func TestSortMulti_EmptySpecs(t *testing.T) {
	entities := []*model.Entity{
		{ID: "C-001"},
		{ID: "A-001"},
	}

	// Should not panic or change order
	SortMulti(entities, nil, nil, nil)
	if entities[0].ID != "C-001" {
		t.Error("SortMulti with nil specs should not change order")
	}

	SortMulti(entities, []model.SortSpec{}, nil, nil)
	if entities[0].ID != "C-001" {
		t.Error("SortMulti with empty specs should not change order")
	}
}

func TestSortMulti_EmptyEntities(_ *testing.T) {
	// Should not panic
	SortMulti(nil, []model.SortSpec{{Property: "id"}}, nil, nil)
	SortMulti([]*model.Entity{}, []model.SortSpec{{Property: "id"}}, nil, nil)
}

func TestSortMulti_CrossTypeSamePropertyType(t *testing.T) {
	// Two different entity types with the same property type — should use type-aware comparison
	entityDefs := map[string]*metamodel.EntityDef{
		"bug": {Properties: map[string]metamodel.PropertyDef{
			"score": {Type: metamodel.PropertyTypeInteger},
		}},
		"feature": {Properties: map[string]metamodel.PropertyDef{
			"score": {Type: metamodel.PropertyTypeInteger},
		}},
	}

	entities := []*model.Entity{
		{ID: "1", Type: "bug", Properties: map[string]interface{}{"score": 10}},
		{ID: "2", Type: "feature", Properties: map[string]interface{}{"score": 3}},
		{ID: "3", Type: "bug", Properties: map[string]interface{}{"score": 7}},
	}

	SortMulti(entities, []model.SortSpec{{Property: "score"}}, entityDefs, nil)

	wantIDs := []string{"2", "3", "1"} // 3, 7, 10
	for i, e := range entities {
		if e.ID != wantIDs[i] {
			t.Errorf("CrossType same prop type [%d] = %s, want %s", i, e.ID, wantIDs[i])
		}
	}
}

func TestSortMulti_CrossTypeDifferentPropertyType(t *testing.T) {
	// Different entity types with different property types for the same name
	// Should compare by type rank: integer (1) < date (2) < boolean (3) < enum (4) < string (5)
	entityDefs := map[string]*metamodel.EntityDef{
		"typeA": {Properties: map[string]metamodel.PropertyDef{
			"value": {Type: metamodel.PropertyTypeString},
		}},
		"typeB": {Properties: map[string]metamodel.PropertyDef{
			"value": {Type: metamodel.PropertyTypeInteger},
		}},
	}

	entities := []*model.Entity{
		{ID: "1", Type: "typeA", Properties: map[string]interface{}{"value": "hello"}},
		{ID: "2", Type: "typeB", Properties: map[string]interface{}{"value": 42}},
	}

	SortMulti(entities, []model.SortSpec{{Property: "value"}}, entityDefs, nil)

	// Integer rank (1) < String rank (5), so typeB entity should come first
	if entities[0].ID != "2" {
		t.Errorf("Expected integer-typed entity first, got %s", entities[0].ID)
	}
	if entities[1].ID != "1" {
		t.Errorf("Expected string-typed entity second, got %s", entities[1].ID)
	}
}

func TestSortMulti_CrossTypeDifferentPropertyTypeSameRankFallback(t *testing.T) {
	// Two different types with same rank — should fall back to string comparison
	entityDefs := map[string]*metamodel.EntityDef{
		"typeA": {Properties: map[string]metamodel.PropertyDef{
			"value": {Type: metamodel.PropertyTypeString},
		}},
		"typeB": {Properties: map[string]metamodel.PropertyDef{
			"value": {Type: metamodel.PropertyTypeString},
		}},
	}

	entities := []*model.Entity{
		{ID: "1", Type: "typeA", Properties: map[string]interface{}{"value": "zebra"}},
		{ID: "2", Type: "typeB", Properties: map[string]interface{}{"value": "alpha"}},
	}

	SortMulti(entities, []model.SortSpec{{Property: "value"}}, entityDefs, nil)

	if entities[0].ID != "2" {
		t.Errorf("Expected 'alpha' first, got entity %s", entities[0].ID)
	}
}

func TestSortMulti_OnlyOneEntityHasPropertyDef(t *testing.T) {
	// One entity type has the property, the other doesn't
	entityDefs := map[string]*metamodel.EntityDef{
		"typeA": {Properties: map[string]metamodel.PropertyDef{
			"priority": {Type: metamodel.PropertyTypeString},
		}},
		"typeB": {Properties: map[string]metamodel.PropertyDef{
			// no "priority" property
		}},
	}

	entities := []*model.Entity{
		{ID: "1", Type: "typeB", Properties: map[string]interface{}{"priority": "low"}},
		{ID: "2", Type: "typeA", Properties: map[string]interface{}{"priority": "high"}},
	}

	SortMulti(entities, []model.SortSpec{{Property: "priority"}}, entityDefs, nil)

	// Entity with property def comes first
	if entities[0].ID != "2" {
		t.Errorf("Expected entity with prop def first, got %s", entities[0].ID)
	}
}

func TestSortMulti_NeitherEntityHasPropertyDef(t *testing.T) {
	// Neither entity type has the property defined — falls back to string comparison
	entityDefs := map[string]*metamodel.EntityDef{
		"typeA": {Properties: map[string]metamodel.PropertyDef{}},
		"typeB": {Properties: map[string]metamodel.PropertyDef{}},
	}

	entities := []*model.Entity{
		{ID: "1", Type: "typeA", Properties: map[string]interface{}{"name": "Zulu"}},
		{ID: "2", Type: "typeB", Properties: map[string]interface{}{"name": "Alpha"}},
	}

	SortMulti(entities, []model.SortSpec{{Property: "name"}}, entityDefs, nil)

	if entities[0].ID != "2" {
		t.Errorf("Expected 'Alpha' first via string comparison, got entity %s", entities[0].ID)
	}
}

func TestSortMulti_NilEntityDefs(t *testing.T) {
	// entityDefs is nil — all properties compared as strings
	entities := []*model.Entity{
		{ID: "1", Type: "item", Properties: map[string]interface{}{"name": "Zulu"}},
		{ID: "2", Type: "item", Properties: map[string]interface{}{"name": "Alpha"}},
	}

	SortMulti(entities, []model.SortSpec{{Property: "name"}}, nil, nil)

	if entities[0].ID != "2" {
		t.Errorf("Expected 'Alpha' first, got entity %s", entities[0].ID)
	}
}

func TestTypeRank(t *testing.T) {
	meta := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"priority": {Values: []string{"high", "low"}},
		},
	}

	tests := []struct {
		name     string
		propDef  *metamodel.PropertyDef
		wantRank int
	}{
		{"integer", &metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger}, typeRankInteger},
		{"date", &metamodel.PropertyDef{Type: metamodel.PropertyTypeDate}, typeRankDate},
		{"boolean", &metamodel.PropertyDef{Type: metamodel.PropertyTypeBoolean}, typeRankBoolean},
		{"enum", &metamodel.PropertyDef{Type: metamodel.PropertyTypeEnum}, typeRankEnum},
		{"string", &metamodel.PropertyDef{Type: metamodel.PropertyTypeString}, typeRankString},
		{"custom type (known)", &metamodel.PropertyDef{Type: "priority"}, typeRankEnum},
		{"custom type (unknown)", &metamodel.PropertyDef{Type: "nonexistent"}, typeRankString},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := typeRank(tt.propDef, meta)
			if got != tt.wantRank {
				t.Errorf("typeRank(%s) = %d, want %d", tt.name, got, tt.wantRank)
			}
		})
	}

	// Test with nil meta — unknown types fall back to string
	got := typeRank(&metamodel.PropertyDef{Type: "priority"}, nil)
	if got != typeRankString {
		t.Errorf("typeRank with nil meta = %d, want %d", got, typeRankString)
	}
}

func TestCompareByPropDef_AllTypes(t *testing.T) {
	// Test date comparison through compareByPropDef
	dateDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeDate, Format: "2006-01-02"}
	if !compareByPropDef("2025-01-01", "2025-06-01", dateDef, nil) {
		t.Error("compareByPropDef date: expected 2025-01-01 < 2025-06-01")
	}

	// Test boolean comparison through compareByPropDef
	boolDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeBoolean}
	if !compareByPropDef(false, true, boolDef, nil) {
		t.Error("compareByPropDef bool: expected false < true")
	}

	// Test custom enum type via default branch
	enumIndex := map[string]int{"high": 0, "medium": 1, "low": 2}
	customDef := &metamodel.PropertyDef{Type: "priority"}
	if !compareByPropDef("high", "low", customDef, enumIndex) {
		t.Error("compareByPropDef custom enum: expected high < low")
	}

	// Test custom type without enum index — falls back to string
	if !compareByPropDef("alpha", "beta", customDef, nil) {
		t.Error("compareByPropDef custom no enum: expected alpha < beta")
	}
}

func TestSortMulti_DescendingProperty(t *testing.T) {
	entityDefs := map[string]*metamodel.EntityDef{
		"item": {Properties: map[string]metamodel.PropertyDef{
			"score": {Type: metamodel.PropertyTypeInteger},
		}},
	}

	entities := []*model.Entity{
		{ID: "1", Type: "item", Properties: map[string]interface{}{"score": 1}},
		{ID: "2", Type: "item", Properties: map[string]interface{}{"score": 3}},
		{ID: "3", Type: "item", Properties: map[string]interface{}{"score": 2}},
	}

	SortMulti(entities, []model.SortSpec{{Property: "score", Direction: "desc"}}, entityDefs, nil)

	wantIDs := []string{"2", "3", "1"} // 3, 2, 1
	for i, e := range entities {
		if e.ID != wantIDs[i] {
			t.Errorf("SortMulti descending [%d] = %s, want %s", i, e.ID, wantIDs[i])
		}
	}
}
