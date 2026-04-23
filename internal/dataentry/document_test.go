package dataentry

import (
	"bytes"
	"context"
	"log/slog"
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
		wantWarn   string // substring expected in warning log; "" means no warning
	}{
		// Form routes: return_to injected verbatim, and a stable id=
		// attribute is added on the <a> so the SPA's click handler has a
		// scroll-back anchor that survives title/content edits.
		//
		// Edit links → id="edit-<entity-id-lowered>-<counter>".
		// Create links → id="create-<form-name-lowered>-<counter>".
		// The counter disambiguates multiple links to the same entity in
		// one document.
		{
			name:       "edit form link with full return path",
			html:       `<a href="/form/full_ticket/TKT-001">Edit</a>`,
			returnPath: "/document/preview?entry=DOC-001",
			expected:   `<a id="edit-tkt-001-0" href="/form/full_ticket/TKT-001?return_to=%2Fdocument%2Fpreview%3Fentry%3DDOC-001">Edit</a>`,
		},
		{
			// Different entities get separately-counted suffixes (each -0).
			name:       "multiple edit form links",
			html:       `<a href="/form/full_ticket/TKT-001">R1</a> and <a href="/form/full_ticket/TKT-002">R2</a>`,
			returnPath: "/doc",
			expected:   `<a id="edit-tkt-001-0" href="/form/full_ticket/TKT-001?return_to=%2Fdoc">R1</a> and <a id="edit-tkt-002-0" href="/form/full_ticket/TKT-002?return_to=%2Fdoc">R2</a>`,
		},
		{
			// Same entity linked twice → counter increments.
			name:       "duplicate edit links to same entity get separate counters",
			html:       `<a href="/form/full_ticket/TKT-001">First</a> and <a href="/form/full_ticket/TKT-001">Second</a>`,
			returnPath: "/doc",
			expected:   `<a id="edit-tkt-001-0" href="/form/full_ticket/TKT-001?return_to=%2Fdoc">First</a> and <a id="edit-tkt-001-1" href="/form/full_ticket/TKT-001?return_to=%2Fdoc">Second</a>`,
		},
		{
			name:       "create form link no query",
			html:       `<a href="/form/full_ticket">Add</a>`,
			returnPath: "/doc",
			expected:   `<a id="create-full_ticket-0" href="/form/full_ticket?return_to=%2Fdoc">Add</a>`,
		},
		{
			name:       "create form link preserves existing query",
			html:       `<a href="/form/full_ticket?prop.status=draft&rel.implements=FEAT-001">Add</a>`,
			returnPath: "/doc",
			expected:   `<a id="create-full_ticket-0" href="/form/full_ticket?prop.status=draft&rel.implements=FEAT-001&return_to=%2Fdoc">Add</a>`,
		},
		{
			name:       "form link preserves fragment",
			html:       `<a href="/form/full_ticket#section">Add</a>`,
			returnPath: "/doc",
			expected:   `<a id="create-full_ticket-0" href="/form/full_ticket?return_to=%2Fdoc#section">Add</a>`,
		},

		// Non-form internal links: unchanged.
		{
			name:       "list link unchanged",
			html:       `<a href="/list/all_tasks">List</a>`,
			returnPath: "/doc",
			expected:   `<a href="/list/all_tasks">List</a>`,
		},
		{
			name:       "entity detail unchanged",
			html:       `<a href="/entity/ticket/TKT-001">Detail</a>`,
			returnPath: "/doc",
			expected:   `<a href="/entity/ticket/TKT-001">Detail</a>`,
		},
		{
			name:       "search with query unchanged",
			html:       `<a href="/search?q=foo">Search</a>`,
			returnPath: "/doc",
			expected:   `<a href="/search?q=foo">Search</a>`,
		},
		{
			name:       "kanban unchanged",
			html:       `<a href="/kanban/sprint">Kanban</a>`,
			returnPath: "/doc",
			expected:   `<a href="/kanban/sprint">Kanban</a>`,
		},

		// External / anchor / mailto: unchanged.
		{
			name:       "external link",
			html:       `<a href="https://example.com">Docs</a>`,
			returnPath: "/doc",
			expected:   `<a href="https://example.com">Docs</a>`,
		},
		{
			name:       "mailto",
			html:       `<a href="mailto:a@b.c">Mail</a>`,
			returnPath: "/doc",
			expected:   `<a href="mailto:a@b.c">Mail</a>`,
		},
		{
			name:       "anchor only",
			html:       `<a href="#section">Jump</a>`,
			returnPath: "/doc",
			expected:   `<a href="#section">Jump</a>`,
		},
		{
			name:       "empty href",
			html:       `<a href="">X</a>`,
			returnPath: "/doc",
			expected:   `<a href="">X</a>`,
		},

		// Legacy schemes: warn + passthrough.
		{
			name:       "legacy edit scheme warns and passes through",
			html:       `<a href="edit://requirement/REQ-001">Edit</a>`,
			returnPath: "/doc",
			expected:   `<a href="edit://requirement/REQ-001">Edit</a>`,
			wantWarn:   "removed scheme",
		},
		{
			name:       "legacy create scheme warns and passes through",
			html:       `<a href="create://requirement?prop.x=1">New</a>`,
			returnPath: "/doc",
			expected:   `<a href="create://requirement?prop.x=1">New</a>`,
			wantWarn:   "removed scheme",
		},

		// Unknown internal path: warn + passthrough.
		{
			name:       "unknown internal path warns and passes through",
			html:       `<a href="/nope/foo">Bogus</a>`,
			returnPath: "/doc",
			expected:   `<a href="/nope/foo">Bogus</a>`,
			wantWarn:   "no matching frontend route",
		},

		// Goldmark emits & as &amp; in href values — rewriter must
		// treat both as pair separators when stripping return_to and
		// preserve the author's encoding on output.
		{
			name:       "goldmark-encoded ampersand in query preserved",
			html:       `<a href="/form/full_ticket?prop.status=draft&amp;rel.implements=FEAT-001">Add</a>`,
			returnPath: "/doc",
			expected:   `<a id="create-full_ticket-0" href="/form/full_ticket?prop.status=draft&amp;rel.implements=FEAT-001&return_to=%2Fdoc">Add</a>`,
		},

		// return_to collision: author-supplied return_to must be
		// stripped and replaced (with a warning) so vue-router doesn't
		// see it as an array.
		{
			name:       "author-supplied return_to stripped and replaced",
			html:       `<a href="/form/full_ticket?return_to=/evil">Trick</a>`,
			returnPath: "/doc",
			expected:   `<a id="create-full_ticket-0" href="/form/full_ticket?return_to=%2Fdoc">Trick</a>`,
			wantWarn:   "reserved key return_to",
		},
		{
			name:       "author-supplied return_to stripped with other params preserved",
			html:       `<a href="/form/full_ticket?a=1&return_to=/evil&b=2">Trick</a>`,
			returnPath: "/doc",
			expected:   `<a id="create-full_ticket-0" href="/form/full_ticket?a=1&b=2&return_to=%2Fdoc">Trick</a>`,
			wantWarn:   "reserved key return_to",
		},
		{
			name:       "author-supplied return_to stripped through goldmark entity",
			html:       `<a href="/form/full_ticket?a=1&amp;return_to=/evil&amp;b=2">Trick</a>`,
			returnPath: "/doc",
			expected:   `<a id="create-full_ticket-0" href="/form/full_ticket?a=1&amp;b=2&return_to=%2Fdoc">Trick</a>`,
			wantWarn:   "reserved key return_to",
		},

		// Empty returnPath: the id= is still emitted (the click handler
		// will use it as the scroll-back anchor) but the href is left
		// untouched — nothing to inject without a returnPath.
		{
			name:       "empty returnPath emits id but skips return_to on create form",
			html:       `<a href="/form/full_ticket?prop.status=open">Add</a>`,
			returnPath: "",
			expected:   `<a id="create-full_ticket-0" href="/form/full_ticket?prop.status=open">Add</a>`,
		},
		{
			name:       "empty returnPath emits id on edit form link",
			html:       `<a href="/form/full_ticket/TKT-001">Edit</a>`,
			returnPath: "",
			expected:   `<a id="edit-tkt-001-0" href="/form/full_ticket/TKT-001">Edit</a>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			result := RewriteDocumentLinks(tt.html, tt.returnPath, routeMatcherFunc(matchFrontendRoute), log)
			if result != tt.expected {
				t.Errorf("RewriteDocumentLinks() =\n%s\nwant:\n%s", result, tt.expected)
			}
			if tt.wantWarn == "" && buf.Len() > 0 {
				t.Errorf("unexpected warning output: %s", buf.String())
			}
			if tt.wantWarn != "" && !strings.Contains(buf.String(), tt.wantWarn) {
				t.Errorf("expected warning containing %q, got: %s", tt.wantWarn, buf.String())
			}
		})
	}
}

func TestDocumentDiskCache(t *testing.T) {
	fs := storage.NewMemFS()
	_ = fs.MkdirAll("/p/.rela", 0o755)
	kvRoot, err := storage.NewRootedFS(fs, "/p/.rela")
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	kv := state.NewFSKV(kvRoot)

	t.Run("cache file naming", func(t *testing.T) {
		entryID := "REQ-001"
		hash := "abc123"
		cacheFile := docCacheSubdir + "/" + entryID + "-" + hash + ".html"
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
		cacheFile1 := docCacheSubdir + "/" + entryID + "-" + hash1 + ".html"
		cacheFile2 := docCacheSubdir + "/" + entryID + "-" + hash2 + ".html"

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
