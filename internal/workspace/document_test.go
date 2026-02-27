package workspace

import (
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestHashEntities(t *testing.T) {
	e1 := &model.Entity{
		ID:   "REQ-001",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title":  "First requirement",
			"status": "draft",
		},
		Content: "Some content",
	}
	e2 := &model.Entity{
		ID:   "REQ-002",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title":  "Second requirement",
			"status": "approved",
		},
		Content: "Other content",
	}

	t.Run("deterministic", func(t *testing.T) {
		hash1 := hashEntities([]*model.Entity{e1, e2})
		hash2 := hashEntities([]*model.Entity{e1, e2})
		if hash1 != hash2 {
			t.Errorf("hash should be deterministic, got %s and %s", hash1, hash2)
		}
	})

	t.Run("order independent", func(t *testing.T) {
		hash1 := hashEntities([]*model.Entity{e1, e2})
		hash2 := hashEntities([]*model.Entity{e2, e1})
		if hash1 != hash2 {
			t.Errorf("hash should be order independent, got %s and %s", hash1, hash2)
		}
	})

	t.Run("content change affects hash", func(t *testing.T) {
		e1Modified := &model.Entity{
			ID:         e1.ID,
			Type:       e1.Type,
			Properties: e1.Properties,
			Content:    "Modified content",
		}
		hash1 := hashEntities([]*model.Entity{e1})
		hash2 := hashEntities([]*model.Entity{e1Modified})
		if hash1 == hash2 {
			t.Errorf("hash should change when content changes")
		}
	})

	t.Run("property change affects hash", func(t *testing.T) {
		e1Modified := &model.Entity{
			ID:   e1.ID,
			Type: e1.Type,
			Properties: map[string]interface{}{
				"title":  "Modified title",
				"status": "draft",
			},
			Content: e1.Content,
		}
		hash1 := hashEntities([]*model.Entity{e1})
		hash2 := hashEntities([]*model.Entity{e1Modified})
		if hash1 == hash2 {
			t.Errorf("hash should change when properties change")
		}
	})

	t.Run("empty entities", func(t *testing.T) {
		hash := hashEntities([]*model.Entity{})
		if hash == "" {
			t.Errorf("hash should not be empty for empty slice")
		}
	})
}

func TestMarkdownToHTML(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		contains []string
	}{
		{
			name:     "basic paragraph",
			markdown: "Hello world",
			contains: []string{"<p>Hello world</p>"},
		},
		{
			name:     "heading",
			markdown: "# Title",
			contains: []string{"<h1", "Title", "</h1>"},
		},
		{
			name:     "bold",
			markdown: "This is **bold** text",
			contains: []string{"<strong>bold</strong>"},
		},
		{
			name:     "link",
			markdown: "[Link](http://example.com)",
			contains: []string{`<a href="http://example.com"`, "Link", "</a>"},
		},
		{
			name:     "code block",
			markdown: "```go\nfunc main() {}\n```",
			contains: []string{"<pre>", "<code", "func main()"},
		},
		{
			name:     "table",
			markdown: "| A | B |\n|---|---|\n| 1 | 2 |",
			contains: []string{"<table>", "<th>", "<td>"},
		},
		{
			name:     "task list",
			markdown: "- [x] Done\n- [ ] Todo",
			contains: []string{"<input", "checked", "disabled"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, err := markdownToHTML(tt.markdown)
			if err != nil {
				t.Fatalf("markdownToHTML failed: %v", err)
			}
			for _, substr := range tt.contains {
				if !strings.Contains(html, substr) {
					t.Errorf("expected HTML to contain %q, got:\n%s", substr, html)
				}
			}
		})
	}
}

func TestRewriteEditLinks(t *testing.T) {
	tests := []struct {
		name       string
		html       string
		returnPath string
		expected   string
	}{
		{
			name:       "basic edit link",
			html:       `<a href="edit://requirement/REQ-001">Edit</a>`,
			returnPath: "/document/preview?entry=DOC-001",
			// Return path is URL-encoded including the hash so browsers send it to the server
			expected: `<a href="/form/requirement/REQ-001?return=%2Fdocument%2Fpreview%3Fentry%3DDOC-001%23req-001">Edit</a>`,
		},
		{
			name:       "multiple edit links",
			html:       `<a href="edit://requirement/REQ-001">R1</a> and <a href="edit://decision/DEC-002">D2</a>`,
			returnPath: "/doc",
			expected:   `<a href="/form/requirement/REQ-001?return=%2Fdoc%23req-001">R1</a> and <a href="/form/decision/DEC-002?return=%2Fdoc%23dec-002">D2</a>`,
		},
		{
			name:       "no edit links",
			html:       `<a href="http://example.com">Normal link</a>`,
			returnPath: "/doc",
			expected:   `<a href="http://example.com">Normal link</a>`,
		},
		{
			name:       "mixed links",
			html:       `<a href="edit://requirement/REQ-001">Edit</a> and <a href="/other">Other</a>`,
			returnPath: "/doc",
			expected:   `<a href="/form/requirement/REQ-001?return=%2Fdoc%23req-001">Edit</a> and <a href="/other">Other</a>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RewriteEditLinks(tt.html, tt.returnPath)
			if result != tt.expected {
				t.Errorf("RewriteEditLinks() =\n%s\nwant:\n%s", result, tt.expected)
			}
		})
	}
}

func TestRewriteCreateLinks(t *testing.T) {
	tests := []struct {
		name       string
		html       string
		returnPath string
		expected   string
	}{
		{
			name:       "basic create link",
			html:       `<a href="create://requirement">Add</a>`,
			returnPath: "/document/preview?entry=DOC-001",
			expected:   `<a href="/form/requirement?return=%2Fdocument%2Fpreview%3Fentry%3DDOC-001">Add</a>`,
		},
		{
			name:       "create link with props",
			html:       `<a href="create://requirement?prop.status=draft">Add</a>`,
			returnPath: "/doc",
			expected:   `<a href="/form/requirement?prop.status=draft&return=%2Fdoc">Add</a>`,
		},
		{
			name:       "create link with props and relations",
			html:       `<a href="create://requirement?prop.status=draft&rel.implements=FEAT-001">Add</a>`,
			returnPath: "/doc",
			expected:   `<a href="/form/requirement?prop.status=draft&rel.implements=FEAT-001&return=%2Fdoc">Add</a>`,
		},
		{
			name:       "multiple create links",
			html:       `<a href="create://requirement">R</a> and <a href="create://decision?prop.status=proposed">D</a>`,
			returnPath: "/doc",
			expected:   `<a href="/form/requirement?return=%2Fdoc">R</a> and <a href="/form/decision?prop.status=proposed&return=%2Fdoc">D</a>`,
		},
		{
			name:       "no create links",
			html:       `<a href="http://example.com">Normal link</a>`,
			returnPath: "/doc",
			expected:   `<a href="http://example.com">Normal link</a>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RewriteCreateLinks(tt.html, tt.returnPath)
			if result != tt.expected {
				t.Errorf("RewriteCreateLinks() =\n%s\nwant:\n%s", result, tt.expected)
			}
		})
	}
}

func TestDocumentCache(t *testing.T) {
	ws := setupTestWorkspace(t)

	// Create a test entity
	_, _, err := ws.CreateEntity("requirement", CreateOptions{
		Properties: map[string]interface{}{
			"title": "Test Requirement",
		},
	})
	if err != nil {
		t.Fatalf("failed to create entity: %v", err)
	}

	t.Run("cache miss then hit", func(t *testing.T) {
		entryID := "REQ-001"
		contentHash := "testhash123"

		// Initially cache should miss
		_, ok := ws.getDocCache(entryID, contentHash)
		if ok {
			t.Error("expected cache miss on first access")
		}

		// Set cache
		ws.setDocCache(entryID, contentHash, "<p>Cached HTML</p>")

		// Now should hit
		html, ok := ws.getDocCache(entryID, contentHash)
		if !ok {
			t.Error("expected cache hit after set")
		}
		if html != "<p>Cached HTML</p>" {
			t.Errorf("unexpected cached HTML: %s", html)
		}
	})

	t.Run("cache invalidation by hash change", func(t *testing.T) {
		entryID := "REQ-002"
		oldHash := "oldhash"
		newHash := "newhash"

		ws.setDocCache(entryID, oldHash, "<p>Old content</p>")

		// Different hash should miss
		_, ok := ws.getDocCache(entryID, newHash)
		if ok {
			t.Error("expected cache miss for different hash")
		}

		// Old hash should still hit
		_, ok = ws.getDocCache(entryID, oldHash)
		if !ok {
			t.Error("expected cache hit for old hash")
		}
	})

	t.Run("explicit invalidation", func(t *testing.T) {
		entryID := "REQ-003"
		hash := "somehash"

		ws.setDocCache(entryID, hash, "<p>Content</p>")

		// Verify it's cached
		_, ok := ws.getDocCache(entryID, hash)
		if !ok {
			t.Fatal("expected cache hit before invalidation")
		}

		// Invalidate
		ws.InvalidateDocumentCache(entryID)

		// Should now miss
		_, ok = ws.getDocCache(entryID, hash)
		if ok {
			t.Error("expected cache miss after invalidation")
		}
	})

	t.Run("invalidate all caches", func(t *testing.T) {
		ws.setDocCache("A", "hash1", "content1")
		ws.setDocCache("B", "hash2", "content2")

		ws.InvalidateAllDocumentCaches()

		_, okA := ws.getDocCache("A", "hash1")
		_, okB := ws.getDocCache("B", "hash2")

		if okA || okB {
			t.Error("expected all caches to be invalidated")
		}
	})
}

func TestDocumentConfig(t *testing.T) {
	t.Run("default timeout", func(t *testing.T) {
		cfg := DocumentConfig{}
		if cfg.Timeout != 0 {
			t.Errorf("expected zero timeout for unset, got %v", cfg.Timeout)
		}
	})

	t.Run("custom timeout", func(t *testing.T) {
		cfg := DocumentConfig{
			Timeout: 60 * time.Second,
		}
		if cfg.Timeout != 60*time.Second {
			t.Errorf("expected 60s timeout, got %v", cfg.Timeout)
		}
	})
}
