package dataentry

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestHashEntities(t *testing.T) {
	e1 := &entity.Entity{
		ID:   "REQ-001",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title":  "First requirement",
			"status": "draft",
		},
		Content: "Some content",
	}
	e2 := &entity.Entity{
		ID:   "REQ-002",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title":  "Second requirement",
			"status": "approved",
		},
		Content: "Other content",
	}

	t.Run("deterministic", func(t *testing.T) {
		hash1 := hashEntities([]*entity.Entity{e1, e2})
		hash2 := hashEntities([]*entity.Entity{e1, e2})
		if hash1 != hash2 {
			t.Errorf("hash should be deterministic, got %s and %s", hash1, hash2)
		}
	})

	t.Run("order independent", func(t *testing.T) {
		hash1 := hashEntities([]*entity.Entity{e1, e2})
		hash2 := hashEntities([]*entity.Entity{e2, e1})
		if hash1 != hash2 {
			t.Errorf("hash should be order independent, got %s and %s", hash1, hash2)
		}
	})

	t.Run("content change affects hash", func(t *testing.T) {
		e1Modified := &entity.Entity{
			ID:         e1.ID,
			Type:       e1.Type,
			Properties: e1.Properties,
			Content:    "Modified content",
		}
		hash1 := hashEntities([]*entity.Entity{e1})
		hash2 := hashEntities([]*entity.Entity{e1Modified})
		if hash1 == hash2 {
			t.Errorf("hash should change when content changes")
		}
	})

	t.Run("property change affects hash", func(t *testing.T) {
		e1Modified := &entity.Entity{
			ID:   e1.ID,
			Type: e1.Type,
			Properties: map[string]interface{}{
				"title":  "Modified title",
				"status": "draft",
			},
			Content: e1.Content,
		}
		hash1 := hashEntities([]*entity.Entity{e1})
		hash2 := hashEntities([]*entity.Entity{e1Modified})
		if hash1 == hash2 {
			t.Errorf("hash should change when properties change")
		}
	})

	t.Run("empty entities", func(t *testing.T) {
		hash := hashEntities([]*entity.Entity{})
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

func TestRewriteDocumentLinks(t *testing.T) {
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
			expected: `<a href="/form/requirement/REQ-001?return_to=%2Fdocument%2Fpreview%3Fentry%3DDOC-001%23req-001">Edit</a>`,
		},
		{
			name:       "multiple edit links",
			html:       `<a href="edit://requirement/REQ-001">R1</a> and <a href="edit://decision/DEC-002">D2</a>`,
			returnPath: "/doc",
			expected:   `<a href="/form/requirement/REQ-001?return_to=%2Fdoc%23req-001">R1</a> and <a href="/form/decision/DEC-002?return_to=%2Fdoc%23dec-002">D2</a>`,
		},
		{
			name:       "edit link mixed with normal",
			html:       `<a href="edit://requirement/REQ-001">Edit</a> and <a href="/other">Other</a>`,
			returnPath: "/doc",
			expected:   `<a href="/form/requirement/REQ-001?return_to=%2Fdoc%23req-001">Edit</a> and <a href="/other">Other</a>`,
		},
		{
			name:       "basic create link",
			html:       `<a href="create://requirement">Add</a>`,
			returnPath: "/document/preview?entry=DOC-001",
			expected:   `<a href="/form/requirement?return_to=%2Fdocument%2Fpreview%3Fentry%3DDOC-001">Add</a>`,
		},
		{
			name:       "create link with props",
			html:       `<a href="create://requirement?prop.status=draft">Add</a>`,
			returnPath: "/doc",
			expected:   `<a href="/form/requirement?prop.status=draft&return_to=%2Fdoc">Add</a>`,
		},
		{
			name:       "create link with props and relations",
			html:       `<a href="create://requirement?prop.status=draft&rel.implements=FEAT-001">Add</a>`,
			returnPath: "/doc",
			expected:   `<a href="/form/requirement?prop.status=draft&rel.implements=FEAT-001&return_to=%2Fdoc">Add</a>`,
		},
		{
			name:       "multiple create links",
			html:       `<a href="create://requirement">R</a> and <a href="create://decision?prop.status=proposed">D</a>`,
			returnPath: "/doc",
			expected:   `<a href="/form/requirement?return_to=%2Fdoc">R</a> and <a href="/form/decision?prop.status=proposed&return_to=%2Fdoc">D</a>`,
		},
		{
			name:       "mixed edit and create links",
			html:       `<a href="edit://requirement/REQ-001">Edit</a> <a href="create://decision">New</a>`,
			returnPath: "/doc",
			expected:   `<a href="/form/requirement/REQ-001?return_to=%2Fdoc%23req-001">Edit</a> <a href="/form/decision?return_to=%2Fdoc">New</a>`,
		},
		{
			name:       "no custom links",
			html:       `<a href="http://example.com">Normal link</a>`,
			returnPath: "/doc",
			expected:   `<a href="http://example.com">Normal link</a>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RewriteDocumentLinks(tt.html, tt.returnPath)
			if result != tt.expected {
				t.Errorf("RewriteDocumentLinks() =\n%s\nwant:\n%s", result, tt.expected)
			}
		})
	}
}

func TestDocumentDiskCache(t *testing.T) {
	fs := storage.NewMemFS()
	_ = fs.MkdirAll("/p/.rela", 0o755)
	kv := state.NewFSKV(fs, "/p/.rela")

	t.Run("cache file naming", func(t *testing.T) {
		entryID := "REQ-001"
		hash := "abc123"
		cacheFile := docCacheDir + "/" + entryID + "-" + hash + ".html"
		content := "<p>Test HTML</p>"

		if err := kv.Put(context.Background(), cacheFile, []byte(content)); err != nil {
			t.Fatalf("failed to write cache file: %v", err)
		}
		data, err := kv.Get(context.Background(), cacheFile)
		if err != nil {
			t.Fatalf("failed to read cache file: %v", err)
		}
		if string(data) != content {
			t.Errorf("expected %q, got %q", content, string(data))
		}
	})

	t.Run("different hash creates different file", func(t *testing.T) {
		entryID := "REQ-002"
		hash1 := "hash1"
		hash2 := "hash2"
		cacheFile1 := docCacheDir + "/" + entryID + "-" + hash1 + ".html"
		cacheFile2 := docCacheDir + "/" + entryID + "-" + hash2 + ".html"

		_ = kv.Put(context.Background(), cacheFile1, []byte("content1"))
		_ = kv.Put(context.Background(), cacheFile2, []byte("content2"))

		data1, _ := kv.Get(context.Background(), cacheFile1)
		data2, _ := kv.Get(context.Background(), cacheFile2)

		if string(data1) != "content1" || string(data2) != "content2" {
			t.Error("cache files should be independent")
		}
	})
}

func TestDocumentRenderConfig(t *testing.T) {
	t.Run("default timeout", func(t *testing.T) {
		cfg := documentRenderConfig{}
		if cfg.Timeout != 0 {
			t.Errorf("expected zero timeout for unset, got %v", cfg.Timeout)
		}
	})

	t.Run("custom timeout", func(t *testing.T) {
		cfg := documentRenderConfig{
			Timeout: 60 * time.Second,
		}
		if cfg.Timeout != 60*time.Second {
			t.Errorf("expected 60s timeout, got %v", cfg.Timeout)
		}
	})
}
