package dataentry

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

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

		// Non-form internal links: get return_to but NO anchor id.
		{
			name:       "list link gets return_to",
			html:       `<a href="/list/all_tasks">List</a>`,
			returnPath: "/doc",
			expected:   `<a href="/list/all_tasks?return_to=%2Fdoc">List</a>`,
		},
		{
			name:       "entity detail gets return_to",
			html:       `<a href="/entity/ticket/TKT-001">Detail</a>`,
			returnPath: "/doc",
			expected:   `<a href="/entity/ticket/TKT-001?return_to=%2Fdoc">Detail</a>`,
		},
		{
			name:       "search preserves existing query and appends return_to",
			html:       `<a href="/search?q=foo">Search</a>`,
			returnPath: "/doc",
			expected:   `<a href="/search?q=foo&return_to=%2Fdoc">Search</a>`,
		},
		{
			name:       "kanban gets return_to",
			html:       `<a href="/kanban/sprint">Kanban</a>`,
			returnPath: "/doc",
			expected:   `<a href="/kanban/sprint?return_to=%2Fdoc">Kanban</a>`,
		},
		{
			name:       "non-form link preserves fragment",
			html:       `<a href="/entity/ticket/TKT-001#notes">Detail</a>`,
			returnPath: "/doc",
			expected:   `<a href="/entity/ticket/TKT-001?return_to=%2Fdoc#notes">Detail</a>`,
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

		// Unknown internal paths get return_to too (the rewriter doesn't
		// try to validate against a route catalog any more). Silent
		// passthrough — no warning.
		{
			name:       "unknown internal path gets return_to",
			html:       `<a href="/nope/foo">Bogus</a>`,
			returnPath: "/doc",
			expected:   `<a href="/nope/foo?return_to=%2Fdoc">Bogus</a>`,
		},

		// Non-form internal link with author-planted return_to: strip,
		// re-inject, warn. Same as form-route case — rewriter owns the key.
		{
			name:       "non-form author-supplied return_to stripped and replaced",
			html:       `<a href="/list/all?return_to=/evil">Trick</a>`,
			returnPath: "/doc",
			expected:   `<a href="/list/all?return_to=%2Fdoc">Trick</a>`,
			wantWarn:   "reserved key return_to",
		},
		{
			name:       "non-form author-supplied return_to stripped with params preserved",
			html:       `<a href="/list/all?a=1&return_to=/evil&b=2">Trick</a>`,
			returnPath: "/doc",
			expected:   `<a href="/list/all?a=1&b=2&return_to=%2Fdoc">Trick</a>`,
			wantWarn:   "reserved key return_to",
		},

		// Empty returnPath on non-form link: strip author-supplied
		// return_to without replacement (single-source-of-truth rule).
		{
			name:       "empty returnPath strips non-form return_to without replacement",
			html:       `<a href="/list/all?a=1&return_to=/evil">Trick</a>`,
			returnPath: "",
			expected:   `<a href="/list/all?a=1">Trick</a>`,
			wantWarn:   "reserved key return_to",
		},
		{
			name:       "empty returnPath leaves clean non-form link untouched",
			html:       `<a href="/entity/ticket/TKT-001">Detail</a>`,
			returnPath: "",
			expected:   `<a href="/entity/ticket/TKT-001">Detail</a>`,
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
			result := RewriteDocumentLinks(tt.html, tt.returnPath, log)
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

// TestRewriteDocumentLinks_AttributeShapes pins the rewriter's tolerance
// for anchor-tag shapes beyond goldmark's default output. Each case
// exercises a different attribute configuration the regex-only rewriter
// (pre-RR-D3K32) would have mishandled; the parseAttrs-based rewriter
// must handle them without leaking duplicate ids or dropping attributes.
func TestRewriteDocumentLinks_AttributeShapes(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	cases := []struct {
		name       string
		html       string
		returnPath string
		expected   string
	}{
		{
			name:       "author-planted id before href is stripped, rewriter owns id",
			html:       `<a id="mine" href="/form/full_ticket/TKT-001">Edit</a>`,
			returnPath: "/doc",
			expected:   `<a id="edit-tkt-001-0" href="/form/full_ticket/TKT-001?return_to=%2Fdoc">Edit</a>`,
		},
		{
			name:       "class before href — class preserved, no duplicate id on form route",
			html:       `<a class="primary" href="/form/full_ticket/TKT-001">Edit</a>`,
			returnPath: "/doc",
			expected:   `<a id="edit-tkt-001-0" class="primary" href="/form/full_ticket/TKT-001?return_to=%2Fdoc">Edit</a>`,
		},
		{
			name:       "id interleaved between class and href is still stripped",
			html:       `<a class="x" id="old" href="/form/full_ticket/TKT-001">Edit</a>`,
			returnPath: "/doc",
			expected:   `<a id="edit-tkt-001-0" class="x" href="/form/full_ticket/TKT-001?return_to=%2Fdoc">Edit</a>`,
		},
		{
			name:       "href before id — order preserved, no duplicate id",
			html:       `<a href="/form/full_ticket/TKT-001" id="old">Edit</a>`,
			returnPath: "/doc",
			expected:   `<a id="edit-tkt-001-0" href="/form/full_ticket/TKT-001?return_to=%2Fdoc">Edit</a>`,
		},
		{
			name:       "extra attributes (title, data-*) preserved on non-form link",
			html:       `<a title="View" data-foo="1" href="/entity/ticket/TKT-001">Detail</a>`,
			returnPath: "/doc",
			expected:   `<a title="View" data-foo="1" href="/entity/ticket/TKT-001?return_to=%2Fdoc">Detail</a>`,
		},
		{
			name:       "single-quoted href",
			html:       `<a href='/list/all_tasks'>List</a>`,
			returnPath: "/doc",
			expected:   `<a href="/list/all_tasks?return_to=%2Fdoc">List</a>`,
		},
		{
			name:       "multiple whitespace between attributes collapses to single space",
			html:       `<a   class="x"    href="/list/all_tasks"  >List</a>`,
			returnPath: "/doc",
			expected:   `<a class="x" href="/list/all_tasks?return_to=%2Fdoc">List</a>`,
		},
		{
			name:       "anchor without href is left untouched (attributes re-serialized)",
			html:       `<a name="anchor-only"></a>`,
			returnPath: "/doc",
			expected:   `<a name="anchor-only"></a>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RewriteDocumentLinks(tc.html, tc.returnPath, log)
			if got != tc.expected {
				t.Errorf("RewriteDocumentLinks() =\n%s\nwant:\n%s", got, tc.expected)
			}
		})
	}
}

// TestRewriteDocumentLinks_Idempotent verifies the rewriter converges on
// rewritten HTML: applying it twice with the same returnPath yields the
// same output as a single pass, and applying it with a second, different
// returnPath strips the first and injects the second. See TKT-JIEKC AC10.
func TestRewriteDocumentLinks_Idempotent(t *testing.T) {
	// Sample HTML covering the interesting path classes: a form route, a
	// non-form internal route, an external link, and a pre-existing
	// fragment.
	in := strings.Join([]string{
		`<a href="/form/full_ticket/TKT-001">Edit</a>`,
		`<a href="/entity/ticket/TKT-001#notes">Detail</a>`,
		`<a href="/list/all_tasks">List</a>`,
		`<a href="https://example.com">External</a>`,
	}, " ")
	log := slog.New(slog.DiscardHandler)

	t.Run("same returnPath converges", func(t *testing.T) {
		first := RewriteDocumentLinks(in, "/doc", log)
		second := RewriteDocumentLinks(first, "/doc", log)
		if first != second {
			t.Errorf("expected byte-equal after second pass\nfirst:  %s\nsecond: %s", first, second)
		}
	})

	t.Run("different returnPath replaces", func(t *testing.T) {
		first := RewriteDocumentLinks(in, "/doc/A", log)
		second := RewriteDocumentLinks(first, "/doc/B", log)
		// Encoded tokens: /doc/A → %2Fdoc%2FA, /doc/B → %2Fdoc%2FB.
		if strings.Contains(second, "%2Fdoc%2FA") {
			t.Errorf("first returnPath /doc/A leaked into second pass: %s", second)
		}
		if !strings.Contains(second, "%2Fdoc%2FB") {
			t.Errorf("expected second returnPath /doc/B (encoded %%2Fdoc%%2FB) in output: %s", second)
		}
		// And only once per link (not two return_to's on the same href).
		occ := strings.Count(second, "return_to=")
		if occ != 3 {
			// Form link + detail link + list link = 3 return_to injections.
			// External is never rewritten.
			t.Errorf("expected 3 return_to= occurrences, got %d: %s", occ, second)
		}

		// The second-pass output itself must also be idempotent —
		// applying the rewriter a third time with the same returnPath
		// must byte-equal the second pass. Guards against a bug where
		// a cleanup-on-strip step leaves a trailing artifact that the
		// next pass would re-consume.
		third := RewriteDocumentLinks(second, "/doc/B", log)
		if second != third {
			t.Errorf("second→third pass not byte-equal with same returnPath\nsecond: %s\nthird:  %s",
				second, third)
		}
	})
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
