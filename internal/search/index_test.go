package search

import (
	"testing"
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

func TestIndex(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	doc := Document{
		ID:      "REQ-001",
		Type:    "requirement",
		Primary: "User Authentication",
		Content: "Users must be able to log in",
	}

	if indexErr := idx.Index(doc); indexErr != nil {
		t.Fatalf("Index() error: %v", indexErr)
	}
}

func TestSearch_ByID(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	doc := Document{
		ID:      "REQ-001",
		Type:    "requirement",
		Primary: "Something",
	}
	if indexErr := idx.Index(doc); indexErr != nil {
		t.Fatalf("Index() error: %v", indexErr)
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

func TestSearch_ByPrimary(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	doc := Document{
		ID:      "REQ-001",
		Type:    "requirement",
		Primary: "User Authentication Feature",
	}
	if indexErr := idx.Index(doc); indexErr != nil {
		t.Fatalf("Index() error: %v", indexErr)
	}

	results, searchErr := idx.Search([]string{"authentication"}, nil, 10)
	if searchErr != nil {
		t.Fatalf("Search() error: %v", searchErr)
	}
	if len(results) == 0 {
		t.Error("expected match by primary field")
	}
}

func TestSearch_Fuzzy(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	doc := Document{
		ID:      "REQ-001",
		Type:    "requirement",
		Primary: "Authentication System",
	}
	if indexErr := idx.Index(doc); indexErr != nil {
		t.Fatalf("Index() error: %v", indexErr)
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

	doc := Document{
		ID:      "REQ-001",
		Type:    "requirement",
		Primary: "Authentication System",
	}
	if indexErr := idx.Index(doc); indexErr != nil {
		t.Fatalf("Index() error: %v", indexErr)
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

	doc := Document{
		ID:      "REQ-001",
		Type:    "requirement",
		Primary: "Something Else",
	}
	if indexErr := idx.Index(doc); indexErr != nil {
		t.Fatalf("Index() error: %v", indexErr)
	}

	results, searchErr := idx.Search([]string{"nonexistent"}, nil, 10)
	if searchErr != nil {
		t.Fatalf("Search() error: %v", searchErr)
	}
	if len(results) != 0 {
		t.Error("expected no match")
	}
}

func TestIndexBatch(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	docs := []Document{
		{ID: "REQ-001", Type: "requirement", Primary: "First"},
		{ID: "REQ-002", Type: "requirement", Primary: "Second"},
	}

	if indexErr := idx.IndexBatch(docs); indexErr != nil {
		t.Fatalf("IndexBatch() error: %v", indexErr)
	}

	results, searchErr := idx.Search([]string{"first"}, nil, 10)
	if searchErr != nil {
		t.Fatalf("Search() error: %v", searchErr)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestRemove(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	doc := Document{
		ID:      "REQ-001",
		Type:    "requirement",
		Primary: "ToBeDeleted",
	}
	if indexErr := idx.Index(doc); indexErr != nil {
		t.Fatalf("Index() error: %v", indexErr)
	}

	// Verify it's found
	results, _ := idx.Search([]string{"tobedeleted"}, nil, 10)
	if len(results) == 0 {
		t.Fatal("document should be found before removal")
	}

	// Remove and verify gone
	if removeErr := idx.Remove("REQ-001"); removeErr != nil {
		t.Fatalf("Remove() error: %v", removeErr)
	}

	results, _ = idx.Search([]string{"tobedeleted"}, nil, 10)
	if len(results) != 0 {
		t.Error("document should not be found after removal")
	}
}

func TestSearch_Phrase(t *testing.T) {
	idx, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error: %v", err)
	}
	defer idx.Close()

	doc1 := Document{
		ID:      "REQ-001",
		Type:    "requirement",
		Primary: "User Authentication System",
	}
	if indexErr := idx.Index(doc1); indexErr != nil {
		t.Fatalf("Index(doc1) error: %v", indexErr)
	}

	doc2 := Document{
		ID:      "REQ-002",
		Type:    "requirement",
		Primary: "User Management and Authentication",
	}
	if indexErr := idx.Index(doc2); indexErr != nil {
		t.Fatalf("Index(doc2) error: %v", indexErr)
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

	doc1 := Document{
		ID:      "REQ-001",
		Type:    "requirement",
		Primary: "User Authentication System Login",
	}
	if indexErr := idx.Index(doc1); indexErr != nil {
		t.Fatalf("Index(doc1) error: %v", indexErr)
	}

	doc2 := Document{
		ID:      "REQ-002",
		Type:    "requirement",
		Primary: "User Authentication System",
	}
	if indexErr := idx.Index(doc2); indexErr != nil {
		t.Fatalf("Index(doc2) error: %v", indexErr)
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

	doc := Document{
		ID:      "REQ-001",
		Type:    "requirement",
		Primary: "Original Title",
	}
	if indexErr := idx.Index(doc); indexErr != nil {
		t.Fatalf("Index() error: %v", indexErr)
	}

	// Verify original is found
	results, _ := idx.Search([]string{"original"}, nil, 10)
	if len(results) == 0 {
		t.Fatal("original title should be found")
	}

	// Update the document
	doc.Primary = "Updated Title"
	if indexErr := idx.Index(doc); indexErr != nil {
		t.Fatalf("Index() error: %v", indexErr)
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

	doc := Document{
		ID:      "REQ-001",
		Type:    "requirement",
		Primary: "Authentication System",
	}
	if indexErr := idx.Index(doc); indexErr != nil {
		t.Fatalf("Index() error: %v", indexErr)
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

	doc := Document{
		ID:      "REQ-001",
		Type:    "requirement",
		Primary: "Authentication System",
	}
	if indexErr := idx.Index(doc); indexErr != nil {
		t.Fatalf("Index() error: %v", indexErr)
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
