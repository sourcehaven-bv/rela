package search

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestNewIndex(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	if idx == nil {
		t.Fatal("NewIndex() returned nil index")
	}
	if closeErr := idx.Close(); closeErr != nil {
		t.Errorf("Close() error: %v", closeErr)
	}
}

func TestIndexEntity(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "User Authentication"
	e.Content = "Users must be able to log in"

	if indexErr := idx.IndexEntity(e); indexErr != nil {
		t.Fatalf("IndexEntity() error: %v", indexErr)
	}
}

func TestSearch_ByID(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "Something"
	if indexErr := idx.IndexEntity(e); indexErr != nil {
		t.Fatalf("IndexEntity() error: %v", indexErr)
	}

	// ID is in the "all" field which uses standard analyzer (case insensitive)
	results, searchErr := idx.Search([]string{"req"}, nil, 10)
	if searchErr != nil {
		t.Fatalf("Search() error: %v", searchErr)
	}
	if len(results) == 0 {
		t.Error("expected match by ID prefix")
	}
}

func TestSearch_ByTitle(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "User Authentication Feature"
	if indexErr := idx.IndexEntity(e); indexErr != nil {
		t.Fatalf("IndexEntity() error: %v", indexErr)
	}

	results, searchErr := idx.Search([]string{"authentication"}, nil, 10)
	if searchErr != nil {
		t.Fatalf("Search() error: %v", searchErr)
	}
	if len(results) == 0 {
		t.Error("expected match by title")
	}
}

func TestSearch_Fuzzy(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "Authentication System"
	if indexErr := idx.IndexEntity(e); indexErr != nil {
		t.Fatalf("IndexEntity() error: %v", indexErr)
	}

	// Typo: "autentication" (missing 'h')
	results, searchErr := idx.Search([]string{"autentication"}, nil, 10)
	if searchErr != nil {
		t.Fatalf("Search() error: %v", searchErr)
	}
	if len(results) == 0 {
		t.Error("expected fuzzy match")
	}
}

func TestSearch_Wildcard(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "Authentication System"
	if indexErr := idx.IndexEntity(e); indexErr != nil {
		t.Fatalf("IndexEntity() error: %v", indexErr)
	}

	results, searchErr := idx.Search([]string{"auth*"}, nil, 10)
	if searchErr != nil {
		t.Fatalf("Search() error: %v", searchErr)
	}
	if len(results) == 0 {
		t.Error("expected wildcard match")
	}
}

func TestSearch_NoMatch(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "Something Else"
	if indexErr := idx.IndexEntity(e); indexErr != nil {
		t.Fatalf("IndexEntity() error: %v", indexErr)
	}

	results, searchErr := idx.Search([]string{"nonexistent"}, nil, 10)
	if searchErr != nil {
		t.Fatalf("Search() error: %v", searchErr)
	}
	if len(results) != 0 {
		t.Error("expected no match")
	}
}

func TestIndexAll(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	entities := []*model.Entity{
		func() *model.Entity {
			e := model.NewEntity("REQ-001", "requirement")
			e.Properties["title"] = "First"
			return e
		}(),
		func() *model.Entity {
			e := model.NewEntity("REQ-002", "requirement")
			e.Properties["title"] = "Second"
			return e
		}(),
	}

	if indexErr := idx.IndexAll(entities); indexErr != nil {
		t.Fatalf("IndexAll() error: %v", indexErr)
	}

	results, searchErr := idx.Search([]string{"first"}, nil, 10)
	if searchErr != nil {
		t.Fatalf("Search() error: %v", searchErr)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestRemoveEntity(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "ToBeDeleted"
	if indexErr := idx.IndexEntity(e); indexErr != nil {
		t.Fatalf("IndexEntity() error: %v", indexErr)
	}

	// Verify it's found
	results, _ := idx.Search([]string{"tobedeleted"}, nil, 10)
	if len(results) == 0 {
		t.Fatal("entity should be found before removal")
	}

	// Remove and verify gone
	if removeErr := idx.RemoveEntity("REQ-001"); removeErr != nil {
		t.Fatalf("RemoveEntity() error: %v", removeErr)
	}

	results, _ = idx.Search([]string{"tobedeleted"}, nil, 10)
	if len(results) != 0 {
		t.Error("entity should not be found after removal")
	}
}

func TestSearch_Phrase(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	e1 := model.NewEntity("REQ-001", "requirement")
	e1.Properties["title"] = "User Authentication System"
	if indexErr := idx.IndexEntity(e1); indexErr != nil {
		t.Fatalf("IndexEntity(e1) error: %v", indexErr)
	}

	e2 := model.NewEntity("REQ-002", "requirement")
	e2.Properties["title"] = "User Management and Authentication"
	if indexErr := idx.IndexEntity(e2); indexErr != nil {
		t.Fatalf("IndexEntity(e2) error: %v", indexErr)
	}

	// Phrase search should only match exact phrase
	results, searchErr := idx.Search(nil, []string{"User Authentication"}, 10)
	if searchErr != nil {
		t.Fatalf("Search() error: %v", searchErr)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 match for exact phrase, got %d", len(results))
	}
	if len(results) > 0 && results[0].ID != "REQ-001" {
		t.Errorf("expected REQ-001 to match, got %s", results[0].ID)
	}
}

func TestSearch_PhraseAndWords(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	e1 := model.NewEntity("REQ-001", "requirement")
	e1.Properties["title"] = "User Authentication System Login"
	if indexErr := idx.IndexEntity(e1); indexErr != nil {
		t.Fatalf("IndexEntity(e1) error: %v", indexErr)
	}

	e2 := model.NewEntity("REQ-002", "requirement")
	e2.Properties["title"] = "User Authentication System"
	if indexErr := idx.IndexEntity(e2); indexErr != nil {
		t.Fatalf("IndexEntity(e2) error: %v", indexErr)
	}

	// Search with words AND phrase - both must match
	results, searchErr := idx.Search([]string{"login"}, []string{"Authentication System"}, 10)
	if searchErr != nil {
		t.Fatalf("Search() error: %v", searchErr)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 match, got %d", len(results))
	}
	if len(results) > 0 && results[0].ID != "REQ-001" {
		t.Errorf("expected REQ-001 to match, got %s", results[0].ID)
	}
}

func TestReindex_Update(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "Original Title"
	if indexErr := idx.IndexEntity(e); indexErr != nil {
		t.Fatalf("IndexEntity() error: %v", indexErr)
	}

	// Verify original is found
	results, _ := idx.Search([]string{"original"}, nil, 10)
	if len(results) == 0 {
		t.Fatal("original title should be found")
	}

	// Update the entity
	e.Properties["title"] = "Updated Title"
	if indexErr := idx.IndexEntity(e); indexErr != nil {
		t.Fatalf("IndexEntity() error: %v", indexErr)
	}

	// Old title should NOT be found
	results, _ = idx.Search([]string{"original"}, nil, 10)
	if len(results) != 0 {
		t.Error("original title should not be found after update")
	}

	// New title SHOULD be found
	results, _ = idx.Search([]string{"updated"}, nil, 10)
	if len(results) == 0 {
		t.Error("updated title should be found")
	}
}

func TestSearchSimple(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "Authentication System"
	if indexErr := idx.IndexEntity(e); indexErr != nil {
		t.Fatalf("IndexEntity() error: %v", indexErr)
	}

	// SearchSimple should work with multi-word query
	results, searchErr := idx.SearchSimple("auth system", 10)
	if searchErr != nil {
		t.Fatalf("SearchSimple() error: %v", searchErr)
	}
	if len(results) == 0 {
		t.Error("expected match")
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	e := model.NewEntity("REQ-001", "requirement")
	e.Properties["title"] = "Authentication System"
	if indexErr := idx.IndexEntity(e); indexErr != nil {
		t.Fatalf("IndexEntity() error: %v", indexErr)
	}

	// Search with different case should still match
	results, searchErr := idx.Search([]string{"AUTHENTICATION"}, nil, 10)
	if searchErr != nil {
		t.Fatalf("Search() error: %v", searchErr)
	}
	if len(results) == 0 {
		t.Error("expected case-insensitive match")
	}
}
