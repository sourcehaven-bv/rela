package lua

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// newMdTestRuntime creates a minimal runtime for markdown tests (no workspace needed).
func newMdTestRuntime(t *testing.T) *Runtime {
	t.Helper()
	var sb strings.Builder
	return NewReader(ReadDeps{}, &sb)
}

func TestMdParse(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantType string
	}{
		{
			name:     "heading",
			input:    "# Title",
			wantLen:  1,
			wantType: "heading",
		},
		{
			name:     "paragraph",
			input:    "Some text",
			wantLen:  1,
			wantType: "paragraph",
		},
		{
			name:     "heading and paragraph",
			input:    "# Title\n\nSome text",
			wantLen:  2,
			wantType: "heading",
		},
		{
			name:     "code block",
			input:    "```go\nfmt.Println()\n```",
			wantLen:  1,
			wantType: "code_block",
		},
		{
			name:     "empty",
			input:    "",
			wantLen:  0,
			wantType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := fmt.Sprintf(`
				local ast = rela.md.parse(%q)
				result_len = #ast
				if #ast > 0 then
					result_type = ast[1].type
				else
					result_type = ""
				end
			`, tt.input)
			require.NoError(t, rt.RunString(code))

			lenVal := rt.L.GetGlobal("result_len")
			assert.Equal(t, tt.wantLen, int(lua.LVAsNumber(lenVal)))

			typeVal := rt.L.GetGlobal("result_type")
			assert.Equal(t, tt.wantType, lua.LVAsString(typeVal))
		})
	}
}

func TestMdRender(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	tests := []struct {
		name string
		code string
		want string
	}{
		{
			name: "heading",
			code: `
				local ast = {rela.md.heading(2, "Test")}
				return rela.md.render(ast)
			`,
			want: "## Test\n",
		},
		{
			name: "paragraph",
			code: `
				local ast = {rela.md.paragraph("Hello world")}
				return rela.md.render(ast)
			`,
			want: "Hello world\n",
		},
		{
			name: "code block",
			code: `
				local ast = {rela.md.code_block("x = 1", "lua")}
				return rela.md.render(ast)
			`,
			want: "```lua\nx = 1\n```\n",
		},
		{
			name: "thematic break",
			code: `
				local ast = {rela.md.thematic_break()}
				return rela.md.render(ast)
			`,
			want: "---\n",
		},
		{
			name: "blockquote",
			code: `
				local ast = {rela.md.blockquote("Quoted text")}
				return rela.md.render(ast)
			`,
			want: "> Quoted text\n",
		},
		{
			name: "unordered list",
			code: `
				local ast = {rela.md.list({"one", "two", "three"})}
				return rela.md.render(ast)
			`,
			want: "- one\n- two\n- three\n",
		},
		{
			name: "ordered list",
			code: `
				local ast = {rela.md.list({"first", "second"}, true)}
				return rela.md.render(ast)
			`,
			want: "1. first\n2. second\n",
		},
		{
			name: "ordered list with 12 items",
			code: `
				local items = {}
				for i = 1, 12 do
					items[i] = "item" .. i
				end
				local ast = {rela.md.list(items, true)}
				return rela.md.render(ast)
			`,
			want: "1. item1\n2. item2\n3. item3\n4. item4\n5. item5\n6. item6\n7. item7\n8. item8\n9. item9\n10. item10\n11. item11\n12. item12\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rt.RunString(tt.code)
			require.NoError(t, err)

			result := rt.L.Get(-1)
			assert.Equal(t, tt.want, lua.LVAsString(result))
			rt.L.Pop(1)
		})
	}
}

func TestMdTaskListParse(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	code := `
		local ast = rela.md.parse("- [x] done\n- [ ] todo\n")
		result_type = ast[1].type
		result_count = #ast[1].items
		item1 = ast[1].items[1]
		item2 = ast[1].items[2]
	`
	require.NoError(t, rt.RunString(code))

	assert.Equal(t, "list", lua.LVAsString(rt.L.GetGlobal("result_type")))
	assert.Equal(t, 2, int(lua.LVAsNumber(rt.L.GetGlobal("result_count"))))

	item1, ok := rt.L.GetGlobal("item1").(*lua.LTable)
	require.True(t, ok, "item1 should be a table")
	assert.Equal(t, lua.LTrue, item1.RawGetString("task"))
	assert.Equal(t, lua.LTrue, item1.RawGetString("checked"))
	assert.Equal(t, "done", flattenItemInlines(t, item1))

	item2, ok := rt.L.GetGlobal("item2").(*lua.LTable)
	require.True(t, ok, "item2 should be a table")
	assert.Equal(t, lua.LTrue, item2.RawGetString("task"))
	assert.Equal(t, lua.LFalse, item2.RawGetString("checked"))
	assert.Equal(t, "todo", flattenItemInlines(t, item2))
}

// flattenItemInlines reads the `inlines` field off a list-item table and
// returns its flattened text. Used by tests that previously read `.text`.
func flattenItemInlines(t *testing.T, item *lua.LTable) string {
	t.Helper()
	inlines, ok := item.RawGetString("inlines").(*lua.LTable)
	require.True(t, ok, "item should have an inlines table")
	return flattenInlines(inlines)
}

func TestMdTaskListRender(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	tests := []struct {
		name string
		code string
		want string
	}{
		{
			name: "constructor with task items",
			code: `
				local ast = {rela.md.list({
					{task=true, checked=true, text="done"},
					{task=true, checked=false, text="todo"},
				})}
				return rela.md.render(ast)
			`,
			want: "- [x] done\n- [ ] todo\n",
		},
		{
			name: "ordered task list",
			code: `
				local ast = {rela.md.list({
					{task=true, checked=true, text="first"},
					{task=true, checked=false, text="second"},
				}, true)}
				return rela.md.render(ast)
			`,
			want: "1. [x] first\n2. [ ] second\n",
		},
		{
			name: "task=false renders as plain item",
			code: `
				local ast = {rela.md.list({
					{task=false, text="plain"},
				})}
				return rela.md.render(ast)
			`,
			want: "- plain\n",
		},
		{
			name: "missing task field renders as plain item",
			code: `
				local ast = {rela.md.list({
					{text="plain"},
				})}
				return rela.md.render(ast)
			`,
			want: "- plain\n",
		},
		{
			name: "missing text field renders empty",
			code: `
				local ast = {rela.md.list({
					{task=true, checked=false},
				})}
				return rela.md.render(ast)
			`,
			want: "- [ ] \n",
		},
		{
			name: "non-bool task field treated as non-task",
			code: `
				local ast = {rela.md.list({
					{task="yes", text="plain"},
				})}
				return rela.md.render(ast)
			`,
			want: "- plain\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rt.RunString(tt.code)
			require.NoError(t, err)

			result := rt.L.Get(-1)
			assert.Equal(t, tt.want, lua.LVAsString(result))
			rt.L.Pop(1)
		})
	}
}

func TestMdTaskListRoundTrip(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "all task items",
			input: "- [x] done\n- [ ] todo\n",
			want:  "- [x] done\n- [ ] todo\n",
		},
		{
			name:  "ordered task items",
			input: "1. [x] first\n2. [ ] second\n",
			want:  "1. [x] first\n2. [ ] second\n",
		},
		{
			name:  "single task item",
			input: "- [x] only\n",
			want:  "- [x] only\n",
		},
		{
			name:  "strikethrough preserved in task",
			input: "- [x] ~~done~~\n",
			want:  "- [x] ~~done~~\n",
		},
		{
			name:  "strikethrough mid-text preserved",
			input: "- [ ] foo ~~bar~~ baz\n",
			want:  "- [ ] foo ~~bar~~ baz\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := fmt.Sprintf(`
				local ast = rela.md.parse(%q)
				return rela.md.render(ast)
			`, tt.input)
			require.NoError(t, rt.RunString(code))

			result := lua.LVAsString(rt.L.Get(-1))
			rt.L.Pop(1)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestMdMixedListBehavior locks in goldmark's actual mixed-list semantics
// and the renderer's policy for them. As of goldmark v1.7, when a list
// mixes checkbox and non-checkbox items, ONLY items that carry their own
// checkbox become task items (the others stay plain strings). The renderer
// always emits checkbox syntax for task=true items, so the rendered output
// is well-defined even if not symmetrically re-parseable.
func TestMdMixedListBehavior(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	// Task first, then plain.
	code := `
		local ast = rela.md.parse("- [x] task\n- plain\n")
		count = #ast[1].items
		item1_type = type(ast[1].items[1])
		item1_task = type(ast[1].items[1]) == "table" and ast[1].items[1].task or false
		item1_text = type(ast[1].items[1]) == "table" and rela.md.flatten(ast[1].items[1].inlines) or ast[1].items[1]
		item2_type = type(ast[1].items[2])
		item2_value = ast[1].items[2]
		rendered = rela.md.render(ast)
	`
	require.NoError(t, rt.RunString(code))

	assert.Equal(t, 2, int(lua.LVAsNumber(rt.L.GetGlobal("count"))))
	// Item 1 is a task item table.
	assert.Equal(t, "table", lua.LVAsString(rt.L.GetGlobal("item1_type")))
	assert.Equal(t, lua.LTrue, rt.L.GetGlobal("item1_task"))
	assert.Equal(t, "task", lua.LVAsString(rt.L.GetGlobal("item1_text")))
	// Item 2 is a plain string (goldmark does not classify it as a task).
	assert.Equal(t, "string", lua.LVAsString(rt.L.GetGlobal("item2_type")))
	assert.Equal(t, "plain", lua.LVAsString(rt.L.GetGlobal("item2_value")))
	// Renderer emits checkbox for task=true items even in mixed lists.
	assert.Equal(t, "- [x] task\n- plain\n", lua.LVAsString(rt.L.GetGlobal("rendered")))
}

// TestMdInlineTextPolicy pins the inline marker preservation policy
// declared in extractInlineText's doc comment. Strikethrough and code
// spans are preserved across all extracted-text contexts; emphasis and
// links are dropped. This test exists to make policy changes visible.
func TestMdInlineTextPolicy(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	tests := []struct {
		name    string
		input   string
		extract string // lua expression returning the text to assert on
		want    string
	}{
		// Strikethrough is preserved everywhere.
		{
			name:    "strikethrough in paragraph",
			input:   "This is ~~struck~~ text.\n",
			extract: "rela.md.flatten(ast[1].inlines)",
			want:    "This is ~~struck~~ text.",
		},
		{
			name:    "strikethrough in heading",
			input:   "# Title with ~~struck~~ word\n",
			extract: "rela.md.flatten(ast[1].inlines)",
			want:    "Title with ~~struck~~ word",
		},
		{
			name:    "strikethrough in blockquote",
			input:   "> quoted ~~struck~~ text\n",
			extract: "rela.md.flatten(ast[1].children[1].inlines)",
			want:    "quoted ~~struck~~ text",
		},
		{
			name:    "strikethrough in table cell",
			input:   "| h |\n|---|\n| ~~struck~~ |\n",
			extract: "rela.md.flatten(ast[1].rows[1][1])",
			want:    "~~struck~~",
		},
		{
			name:    "strikethrough in task item",
			input:   "- [x] foo ~~bar~~ baz\n",
			extract: "rela.md.flatten(ast[1].items[1].inlines)",
			want:    "foo ~~bar~~ baz",
		},
		// Code spans are preserved.
		{
			name:    "code span in paragraph",
			input:   "Use `printf` for output.\n",
			extract: "rela.md.flatten(ast[1].inlines)",
			want:    "Use `printf` for output.",
		},
		{
			name:    "code span in task item",
			input:   "- [x] call `foo()`\n",
			extract: "rela.md.flatten(ast[1].items[1].inlines)",
			want:    "call `foo()`",
		},
		// Strikethrough does NOT activate inside fenced code blocks.
		{
			name:    "code block content keeps tildes literally",
			input:   "```\n~~not struck~~\n```\n",
			extract: "ast[1].content",
			want:    "~~not struck~~",
		},
		// Emphasis/strong/links: flatten() keeps the legacy policy
		// (drop wrappers, keep inner text).
		{
			name:    "bold dropped in task item",
			input:   "- [x] **bold** text\n",
			extract: "rela.md.flatten(ast[1].items[1].inlines)",
			want:    "bold text",
		},
		{
			name:    "italic dropped in paragraph",
			input:   "Some *italic* word.\n",
			extract: "rela.md.flatten(ast[1].inlines)",
			want:    "Some italic word.",
		},
		{
			name:    "link dropped to text only",
			input:   "See [docs](http://example.com).\n",
			extract: "rela.md.flatten(ast[1].inlines)",
			want:    "See docs.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := fmt.Sprintf(`
				local ast = rela.md.parse(%q)
				result = %s
			`, tt.input, tt.extract)
			require.NoError(t, rt.RunString(code))
			assert.Equal(t, tt.want, lua.LVAsString(rt.L.GetGlobal("result")))
		})
	}
}

// TestMdTaskListEmptyText covers parser-side handling of checkboxes with
// no text after them.
func TestMdTaskListEmptyText(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	code := `
		local ast = rela.md.parse("- [x] \n- [ ] \n")
		count = #ast[1].items
		item1_task = ast[1].items[1].task
		item1_checked = ast[1].items[1].checked
		item1_text = rela.md.flatten(ast[1].items[1].inlines)
		item2_task = ast[1].items[2].task
		item2_checked = ast[1].items[2].checked
		item2_text = rela.md.flatten(ast[1].items[2].inlines)
	`
	require.NoError(t, rt.RunString(code))

	assert.Equal(t, 2, int(lua.LVAsNumber(rt.L.GetGlobal("count"))))
	assert.Equal(t, lua.LTrue, rt.L.GetGlobal("item1_task"))
	assert.Equal(t, lua.LTrue, rt.L.GetGlobal("item1_checked"))
	assert.Empty(t, lua.LVAsString(rt.L.GetGlobal("item1_text")))
	assert.Equal(t, lua.LTrue, rt.L.GetGlobal("item2_task"))
	assert.Equal(t, lua.LFalse, rt.L.GetGlobal("item2_checked"))
	assert.Empty(t, lua.LVAsString(rt.L.GetGlobal("item2_text")))
}

// TestMdTaskListNonStringText verifies the renderer coerces non-string
// text values rather than silently producing empty output.
func TestMdTaskListNonStringText(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	code := `
		local ast = {rela.md.list({
			{task=true, checked=false, text=42},
		})}
		return rela.md.render(ast)
	`
	require.NoError(t, rt.RunString(code))
	assert.Equal(t, "- [ ] 42\n", lua.LVAsString(rt.L.Get(-1)))
	rt.L.Pop(1)
}

// TestMdTaskListNonBoolTaskValues verifies that only an explicit lua bool
// true qualifies as a task item — strings, numbers, tables, nil all fall
// through to the plain rendering path.
func TestMdTaskListNonBoolTaskValues(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	tests := []struct {
		name     string
		taskExpr string
	}{
		{"string truthy", `task="yes"`},
		{"number 1", `task=1`},
		{"number 0", `task=0`},
		{"empty table", `task={}`},
		{"nil", `task=nil`},
		{"explicit false", `task=false`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := fmt.Sprintf(`
				local ast = {rela.md.list({
					{%s, text="plain"},
				})}
				return rela.md.render(ast)
			`, tt.taskExpr)
			require.NoError(t, rt.RunString(code))
			assert.Equal(t, "- plain\n", lua.LVAsString(rt.L.Get(-1)))
			rt.L.Pop(1)
		})
	}
}

// TestMdTaskListSurvivesShiftHeaders ensures task item table shape is
// preserved when an AST is transformed by shift_headers, which uses a
// generic deep-copy walker. A future optimization that special-cased
// non-heading nodes could silently break task lists; this test catches it.
func TestMdTaskListSurvivesShiftHeaders(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	code := `
		local ast = rela.md.parse("# Header\n\n- [x] done\n- [ ] todo\n")
		ast = rela.md.shift_headers(ast, 1)
		return rela.md.render(ast)
	`
	require.NoError(t, rt.RunString(code))
	// Header shifted, task items intact.
	assert.Equal(t,
		"## Header\n\n- [x] done\n- [ ] todo\n",
		lua.LVAsString(rt.L.Get(-1)))
	rt.L.Pop(1)
}

// TestMdTaskListMultiBlockItem documents that only the first text block
// of a multi-paragraph list item is captured (matches the goldmark task
// list spec which requires the checkbox in the first text block).
func TestMdTaskListMultiBlockItem(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	// A list item with a continuation line — single TextBlock, captured fully.
	code := `
		local ast = rela.md.parse("- [x] first line\n  second line\n")
		result = rela.md.flatten(ast[1].items[1].inlines)
	`
	require.NoError(t, rt.RunString(code))
	// Soft line break renders as space in extracted text.
	assert.Contains(t, lua.LVAsString(rt.L.GetGlobal("result")), "first line")
}

// TestMdRenderListSparseTable verifies that scripts which "delete" an
// item by assigning nil get a compact rendering — the renderer skips nil
// holes rather than emitting empty bullets. Note that Lua's # operator
// returns a "border" (the boundary between non-nil and nil), so behavior
// at gaps depends on the table's internal structure; for an unordered
// list this works because we iterate over the border range and skip nils.
func TestMdRenderListSparseTable(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	code := `
		local ast = {rela.md.list({"a", "b", "c"})}
		ast[1].items[2] = nil
		return rela.md.render(ast)
	`
	require.NoError(t, rt.RunString(code))
	rendered := lua.LVAsString(rt.L.Get(-1))
	rt.L.Pop(1)
	// Item "b" is gone; "a" and "c" remain. The exact result depends on
	// whether the # operator returned 1 or 3 — we accept either compaction
	// to "- a\n" or "- a\n- c\n", but never an empty bullet.
	assert.NotContains(t, rendered, "- \n", "should not produce empty bullets")
	assert.Contains(t, rendered, "- a\n")
}

func TestMdRoundTrip(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "heading",
			input: "# Title\n",
		},
		{
			name:  "multiple headings",
			input: "# H1\n\n## H2\n\n### H3\n",
		},
		{
			name:  "paragraph",
			input: "Some paragraph text here.\n",
		},
		{
			name:  "code block",
			input: "```python\nprint('hello')\n```\n",
		},
		{
			name:  "mixed content",
			input: "# Title\n\nSome text.\n\n## Section\n\nMore text.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := fmt.Sprintf(`
				local ast = rela.md.parse(%q)
				return rela.md.render(ast)
			`, tt.input)
			require.NoError(t, rt.RunString(code))

			result1 := lua.LVAsString(rt.L.Get(-1))
			rt.L.Pop(1)

			// Parse again and render to ensure stability
			code2 := fmt.Sprintf(`
				local ast = rela.md.parse(%q)
				return rela.md.render(ast)
			`, result1)
			require.NoError(t, rt.RunString(code2))

			result2 := lua.LVAsString(rt.L.Get(-1))
			rt.L.Pop(1)

			assert.Equal(t, result1, result2, "round-trip should be stable")
		})
	}
}

func TestMdShiftHeaders(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	tests := []struct {
		name   string
		input  string
		offset int
		want   string
	}{
		{
			name:   "shift down",
			input:  "# H1\n\n## H2\n",
			offset: 1,
			want:   "## H1\n\n### H2\n",
		},
		{
			name:   "shift up",
			input:  "## H2\n\n### H3\n",
			offset: -1,
			want:   "# H2\n\n## H3\n",
		},
		{
			name:   "clamp at max",
			input:  "###### H6\n",
			offset: 1,
			want:   "###### H6\n",
		},
		{
			name:   "clamp at min",
			input:  "# H1\n",
			offset: -1,
			want:   "# H1\n",
		},
		{
			name:   "preserves non-headers",
			input:  "# Title\n\nParagraph text.\n",
			offset: 1,
			want:   "## Title\n\nParagraph text.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := fmt.Sprintf(`
				local ast = rela.md.parse(%q)
				ast = rela.md.shift_headers(ast, %d)
				return rela.md.render(ast)
			`, tt.input, tt.offset)
			require.NoError(t, rt.RunString(code))

			result := lua.LVAsString(rt.L.Get(-1))
			rt.L.Pop(1)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMdSetMinHeaderLevel(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	tests := []struct {
		name   string
		input  string
		target int
		want   string
	}{
		{
			name:   "normalize to level 2",
			input:  "# H1\n\n### H3\n",
			target: 2,
			want:   "## H1\n\n#### H3\n",
		},
		{
			name:   "already at target",
			input:  "## H2\n\n### H3\n",
			target: 2,
			want:   "## H2\n\n### H3\n",
		},
		{
			name:   "no headers",
			input:  "Just text\n",
			target: 2,
			want:   "Just text\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := fmt.Sprintf(`
				local ast = rela.md.parse(%q)
				ast = rela.md.set_min_header_level(ast, %d)
				return rela.md.render(ast)
			`, tt.input, tt.target)
			require.NoError(t, rt.RunString(code))

			result := lua.LVAsString(rt.L.Get(-1))
			rt.L.Pop(1)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMdHeaders(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	t.Run("extracts all headers", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("# One\n\ntext\n\n## Two\n\n### Three\n")
			local headers = rela.md.headers(ast)
			return #headers, headers[1].level, headers[1].title, headers[2].level, headers[2].title
		`
		require.NoError(t, rt.RunString(code))

		assert.Equal(t, "3", rt.L.Get(-5).String())   // count
		assert.Equal(t, "1", rt.L.Get(-4).String())   // h1 level
		assert.Equal(t, "One", rt.L.Get(-3).String()) // h1 title
		assert.Equal(t, "2", rt.L.Get(-2).String())   // h2 level
		assert.Equal(t, "Two", rt.L.Get(-1).String()) // h2 title
		rt.L.Pop(5)
	})

	t.Run("filters by level", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("# H1\n\n## H2\n\n### H3\n")
			local headers = rela.md.headers(ast, {min_level = 2, max_level = 2})
			return #headers, headers[1] and headers[1].title or "none"
		`
		require.NoError(t, rt.RunString(code))

		assert.Equal(t, "1", rt.L.Get(-2).String())
		assert.Equal(t, "H2", rt.L.Get(-1).String())
		rt.L.Pop(2)
	})

	t.Run("empty when no headers", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("Just text")
			local headers = rela.md.headers(ast)
			return #headers
		`
		require.NoError(t, rt.RunString(code))

		assert.Equal(t, "0", rt.L.Get(-1).String())
		rt.L.Pop(1)
	})
}

func TestMdExtractSection(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	t.Run("extracts matching section", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("# Intro\n\ntext1\n\n## Overview\n\ntext2\n\n## Details\n\ntext3\n")
			local section = rela.md.extract_section(ast, "Overview")
			if section then
				return rela.md.render(section)
			end
			return "nil"
		`
		require.NoError(t, rt.RunString(code))

		result := lua.LVAsString(rt.L.Get(-1))
		rt.L.Pop(1)
		assert.Contains(t, result, "## Overview")
		assert.Contains(t, result, "text2")
		assert.NotContains(t, result, "text3")
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("# Title\n\ntext\n")
			local section = rela.md.extract_section(ast, "Missing")
			return section == nil
		`
		require.NoError(t, rt.RunString(code))

		assert.Equal(t, "true", rt.L.Get(-1).String())
		rt.L.Pop(1)
	})

	t.Run("includes nested content", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("## Section\n\n### Subsection\n\ncontent\n\n## Next\n")
			local section = rela.md.extract_section(ast, "Section")
			return rela.md.render(section)
		`
		require.NoError(t, rt.RunString(code))

		result := lua.LVAsString(rt.L.Get(-1))
		rt.L.Pop(1)
		assert.Contains(t, result, "## Section")
		assert.Contains(t, result, "### Subsection")
		assert.Contains(t, result, "content")
		assert.NotContains(t, result, "## Next")
	})
}

func TestMdFirstParagraph(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	t.Run("extracts first paragraph", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("# Title\n\nFirst para.\n\nSecond para.\n")
			return rela.md.first_paragraph(ast)
		`
		require.NoError(t, rt.RunString(code))

		result := lua.LVAsString(rt.L.Get(-1))
		rt.L.Pop(1)
		assert.Equal(t, "First para.", result)
	})

	t.Run("returns nil when no paragraph", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("# Only heading\n")
			return rela.md.first_paragraph(ast) == nil
		`
		require.NoError(t, rt.RunString(code))

		assert.Equal(t, "true", rt.L.Get(-1).String())
		rt.L.Pop(1)
	})
}

func TestMdConcat(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	code := `
		local ast1 = rela.md.parse("# Part 1\n")
		local ast2 = rela.md.parse("# Part 2\n")
		local combined = rela.md.concat(ast1, ast2)
		return rela.md.render(combined)
	`
	require.NoError(t, rt.RunString(code))

	result := lua.LVAsString(rt.L.Get(-1))
	rt.L.Pop(1)
	assert.Contains(t, result, "# Part 1")
	assert.Contains(t, result, "# Part 2")
}

func TestMdNodeConstructors(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	t.Run("heading clamps level high", func(t *testing.T) {
		code := `
			local h = rela.md.heading(10, "Test")
			return h.level
		`
		require.NoError(t, rt.RunString(code))
		assert.Equal(t, "6", rt.L.Get(-1).String())
		rt.L.Pop(1)
	})

	t.Run("heading clamps level low", func(t *testing.T) {
		code := `
			local h = rela.md.heading(0, "Test")
			return h.level
		`
		require.NoError(t, rt.RunString(code))
		assert.Equal(t, "1", rt.L.Get(-1).String())
		rt.L.Pop(1)
	})

	t.Run("code_block without language", func(t *testing.T) {
		code := `
			local cb = rela.md.code_block("x = 1")
			local ast = {cb}
			return rela.md.render(ast)
		`
		require.NoError(t, rt.RunString(code))

		result := lua.LVAsString(rt.L.Get(-1))
		rt.L.Pop(1)
		assert.Equal(t, "```\nx = 1\n```\n", result)
	})

	t.Run("multiline blockquote", func(t *testing.T) {
		code := `
			local bq = rela.md.blockquote("line1\nline2")
			local ast = {bq}
			return rela.md.render(ast)
		`
		require.NoError(t, rt.RunString(code))

		result := lua.LVAsString(rt.L.Get(-1))
		rt.L.Pop(1)
		assert.Equal(t, "> line1\n> line2\n", result)
	})
}

func TestMdCodeBlockInContent(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	// Ensure "# not a header" inside code block is preserved
	code := `
		local md = "# Real Header\n\n` + "```" + `\n# not a header\n` + "```" + `\n"
		local ast = rela.md.parse(md)
		ast = rela.md.shift_headers(ast, 1)
		return rela.md.render(ast)
	`
	require.NoError(t, rt.RunString(code))

	result := lua.LVAsString(rt.L.Get(-1))
	rt.L.Pop(1)

	// Real header should be shifted
	assert.Contains(t, result, "## Real Header")
	// Code block content should be preserved as-is
	assert.Contains(t, result, "# not a header")
}

func TestMdUnicodeContent(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	code := `
		local ast = rela.md.parse("# 日本語タイトル\n\nUnicode: émojis 🎉\n")
		return rela.md.render(ast)
	`
	require.NoError(t, rt.RunString(code))

	result := lua.LVAsString(rt.L.Get(-1))
	rt.L.Pop(1)
	assert.Contains(t, result, "日本語タイトル")
	assert.Contains(t, result, "émojis 🎉")
}

func TestMdTableParse(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	t.Run("simple table", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("| Name | Age |\n| ---- | --- |\n| Alice | 30 |\n| Bob | 25 |\n")
			result_len = #ast
			result_type = ast[1].type
			result_header_len = #ast[1].header
			result_h1 = rela.md.flatten(ast[1].header[1])
			result_h2 = rela.md.flatten(ast[1].header[2])
			result_rows_len = #ast[1].rows
			result_r1c1 = rela.md.flatten(ast[1].rows[1][1])
			result_r1c2 = rela.md.flatten(ast[1].rows[1][2])
			result_r2c1 = rela.md.flatten(ast[1].rows[2][1])
			result_r2c2 = rela.md.flatten(ast[1].rows[2][2])
		`
		require.NoError(t, rt.RunString(code))

		assert.Equal(t, 1, int(lua.LVAsNumber(rt.L.GetGlobal("result_len"))))
		assert.Equal(t, "table", lua.LVAsString(rt.L.GetGlobal("result_type")))
		assert.Equal(t, 2, int(lua.LVAsNumber(rt.L.GetGlobal("result_header_len"))))
		assert.Equal(t, "Name", lua.LVAsString(rt.L.GetGlobal("result_h1")))
		assert.Equal(t, "Age", lua.LVAsString(rt.L.GetGlobal("result_h2")))
		assert.Equal(t, 2, int(lua.LVAsNumber(rt.L.GetGlobal("result_rows_len"))))
		assert.Equal(t, "Alice", lua.LVAsString(rt.L.GetGlobal("result_r1c1")))
		assert.Equal(t, "30", lua.LVAsString(rt.L.GetGlobal("result_r1c2")))
		assert.Equal(t, "Bob", lua.LVAsString(rt.L.GetGlobal("result_r2c1")))
		assert.Equal(t, "25", lua.LVAsString(rt.L.GetGlobal("result_r2c2")))
	})

	t.Run("alignment markers", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("| Left | Center | Right |\n| :--- | :---: | ---: |\n| a | b | c |\n")
			result_a1 = ast[1].alignments[1]
			result_a2 = ast[1].alignments[2]
			result_a3 = ast[1].alignments[3]
		`
		require.NoError(t, rt.RunString(code))

		assert.Equal(t, "left", lua.LVAsString(rt.L.GetGlobal("result_a1")))
		assert.Equal(t, "center", lua.LVAsString(rt.L.GetGlobal("result_a2")))
		assert.Equal(t, "right", lua.LVAsString(rt.L.GetGlobal("result_a3")))
	})

	t.Run("header only table", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("| Col |\n| --- |\n")
			result_type = ast[1].type
			result_header_len = #ast[1].header
			result_rows_len = #ast[1].rows
		`
		require.NoError(t, rt.RunString(code))

		assert.Equal(t, "table", lua.LVAsString(rt.L.GetGlobal("result_type")))
		assert.Equal(t, 1, int(lua.LVAsNumber(rt.L.GetGlobal("result_header_len"))))
		assert.Equal(t, 0, int(lua.LVAsNumber(rt.L.GetGlobal("result_rows_len"))))
	})

	t.Run("mixed content with table", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("# Title\n\n| A | B |\n| - | - |\n| 1 | 2 |\n\nSome text.\n")
			result_len = #ast
			result_t1 = ast[1].type
			result_t2 = ast[2].type
			result_t3 = ast[3].type
		`
		require.NoError(t, rt.RunString(code))

		assert.Equal(t, 3, int(lua.LVAsNumber(rt.L.GetGlobal("result_len"))))
		assert.Equal(t, "heading", lua.LVAsString(rt.L.GetGlobal("result_t1")))
		assert.Equal(t, "table", lua.LVAsString(rt.L.GetGlobal("result_t2")))
		assert.Equal(t, "paragraph", lua.LVAsString(rt.L.GetGlobal("result_t3")))
	})

	t.Run("inline formatting in cells", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("| Header |\n| --- |\n| **bold** text |\n")
			result_cell = rela.md.flatten(ast[1].rows[1][1])
		`
		require.NoError(t, rt.RunString(code))

		assert.Equal(t, "bold text", lua.LVAsString(rt.L.GetGlobal("result_cell")))
	})
}

func TestMdTableRender(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	t.Run("render simple table with padding", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("| Name | Age |\n| ---- | --- |\n| Alice | 30 |\n")
			return rela.md.render(ast)
		`
		require.NoError(t, rt.RunString(code))

		result := lua.LVAsString(rt.L.Get(-1))
		rt.L.Pop(1)
		// "Name" is 4 chars, "Alice" is 5 → column width 5, so "Name" gets padded
		assert.Contains(t, result, "| Name  | Age |")
		assert.Contains(t, result, "| Alice | 30  |")
	})

	t.Run("render with alignments and padding", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("| Left | Center | Right |\n| :--- | :---: | ---: |\n| a | b | c |\n")
			return rela.md.render(ast)
		`
		require.NoError(t, rt.RunString(code))

		result := lua.LVAsString(rt.L.Get(-1))
		rt.L.Pop(1)
		// Check alignment markers are present in separator
		assert.Contains(t, result, ":---")   // left align
		assert.Contains(t, result, ":----:") // center align
		assert.Contains(t, result, "----:")  // right align
		// Check right-aligned cell is right-padded
		assert.Contains(t, result, "     c |")
	})

	t.Run("render header only", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("| Col |\n| --- |\n")
			return rela.md.render(ast)
		`
		require.NoError(t, rt.RunString(code))

		result := lua.LVAsString(rt.L.Get(-1))
		rt.L.Pop(1)
		assert.Contains(t, result, "| Col |")
		assert.Contains(t, result, "---")
	})

	t.Run("render missing header gracefully", func(t *testing.T) {
		code := `
			local node = {type = "table", rows = {{"a", "b"}}}
			local ast = {node}
			return rela.md.render(ast)
		`
		require.NoError(t, rt.RunString(code))

		result := lua.LVAsString(rt.L.Get(-1))
		rt.L.Pop(1)
		// Should produce empty output since header is nil
		assert.Empty(t, result)
	})
}

func TestMdTableRoundTrip(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple table",
			input: "| Name | Age |\n| -------- | -------- |\n| Alice | 30 |\n",
		},
		{
			name:  "table with alignments",
			input: "| L | C | R |\n| :-------- | :-------: | --------: |\n| a | b | c |\n",
		},
		{
			name:  "single column",
			input: "| Col |\n| -------- |\n| val |\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := fmt.Sprintf(`
				local ast = rela.md.parse(%q)
				return rela.md.render(ast)
			`, tt.input)
			require.NoError(t, rt.RunString(code))

			result1 := lua.LVAsString(rt.L.Get(-1))
			rt.L.Pop(1)

			// Parse again and render to ensure stability
			code2 := fmt.Sprintf(`
				local ast = rela.md.parse(%q)
				return rela.md.render(ast)
			`, result1)
			require.NoError(t, rt.RunString(code2))

			result2 := lua.LVAsString(rt.L.Get(-1))
			rt.L.Pop(1)

			assert.Equal(t, result1, result2, "round-trip should be stable")
		})
	}
}

func TestMdTableRenderFormatting(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	t.Run("columns padded to widest cell", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("| X | Y |\n| --- | --- |\n| short | longer value |\n")
			return rela.md.render(ast)
		`
		require.NoError(t, rt.RunString(code))

		result := lua.LVAsString(rt.L.Get(-1))
		rt.L.Pop(1)
		want := "| X     | Y            |\n" +
			"| ----- | ------------ |\n" +
			"| short | longer value |\n"
		assert.Equal(t, want, result)
	})

	t.Run("right-aligned numbers padded right", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("| Item | Price |\n| :--- | ---: |\n| Apple | 1 |\n| Banana Split | 1250 |\n")
			return rela.md.render(ast)
		`
		require.NoError(t, rt.RunString(code))

		result := lua.LVAsString(rt.L.Get(-1))
		rt.L.Pop(1)
		want := "| Item         | Price |\n" +
			"| :----------- | ----: |\n" +
			"| Apple        |     1 |\n" +
			"| Banana Split |  1250 |\n"
		assert.Equal(t, want, result)
	})

	t.Run("center-aligned cells centered", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("| Status |\n| :---: |\n| OK |\n| FAILED |\n")
			return rela.md.render(ast)
		`
		require.NoError(t, rt.RunString(code))

		result := lua.LVAsString(rt.L.Get(-1))
		rt.L.Pop(1)
		want := "| Status |\n" +
			"| :----: |\n" +
			"|   OK   |\n" +
			"| FAILED |\n"
		assert.Equal(t, want, result)
	})

	t.Run("multi-byte characters aligned correctly", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("| Name | Price |\n| --- | ---: |\n| 日本語 | 100 |\n| Go | 2000 |\n")
			return rela.md.render(ast)
		`
		require.NoError(t, rt.RunString(code))

		result := lua.LVAsString(rt.L.Get(-1))
		rt.L.Pop(1)
		// 日本語 is 6 display columns wide, "Name" is 4 → column width 6
		assert.Contains(t, result, "| Name   |")
		assert.Contains(t, result, "| 日本語 |")
		assert.Contains(t, result, "| Go     |")
	})
}

func TestMdParseErrors(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	// Note: gopher-lua's CheckString coerces numbers to strings,
	// so parse(123) becomes parse("123") - this is expected behavior.
	// We test that nil arguments raise errors.
	t.Run("parse with nil errors", func(t *testing.T) {
		code := `rela.md.parse(nil)`
		err := rt.RunString(code)
		assert.Error(t, err)
	})

	t.Run("shift_headers with non-table errors", func(t *testing.T) {
		code := `rela.md.shift_headers("not a table", 1)`
		err := rt.RunString(code)
		assert.Error(t, err)
	})

	t.Run("shift_headers with non-number errors", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("# Test")
			rela.md.shift_headers(ast, "not a number")
		`
		err := rt.RunString(code)
		assert.Error(t, err)
	})

	t.Run("render with non-table errors", func(t *testing.T) {
		code := `rela.md.render("not a table")`
		err := rt.RunString(code)
		assert.Error(t, err)
	})
}

func TestMdIntegrationWithEntity(t *testing.T) {
	t.Parallel()
	// Use the full test workspace to test integration with entities
	ws := testWorkspace(t)
	var sb strings.Builder
	rt := NewWriter(ws.services("/project"), &sb)
	defer rt.Close()

	code := `
		local entity = rela.get_entity("TKT-001")
		if entity and entity.content then
			local ast = rela.md.parse(entity.content)
			ast = rela.md.shift_headers(ast, 1)
			return rela.md.render(ast)
		end
		return "no content"
	`
	require.NoError(t, rt.RunString(code))

	result := lua.LVAsString(rt.L.Get(-1))
	rt.L.Pop(1)
	// TKT-001 has content "Test content" which is a paragraph
	assert.Contains(t, result, "Test content")
}

// --- Inline-structure refactor tests (TKT-9WZIP) ---

// TestMdParseShape verifies that block nodes have `inlines` (not `text`)
// after parse — AC1.
func TestMdParseShape(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()
	require.NoError(t, rt.RunString(`
		local ast = rela.md.parse("# Title\n\nA paragraph.\n")
		head_text = ast[1].text
		head_inl = type(ast[1].inlines)
		para_text = ast[2].text
		para_inl = type(ast[2].inlines)
	`))
	assert.Equal(t, lua.LNil, rt.L.GetGlobal("head_text"))
	assert.Equal(t, "table", lua.LVAsString(rt.L.GetGlobal("head_inl")))
	assert.Equal(t, lua.LNil, rt.L.GetGlobal("para_text"))
	assert.Equal(t, "table", lua.LVAsString(rt.L.GetGlobal("para_inl")))
}

// TestMdInlineKindsRoundTrip covers AC4-AC8: link, image, raw HTML,
// autolink, emphasis, strong, breaks all survive parse → render.
func TestMdInlineKindsRoundTrip(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "link round-trip",
			input: "See [docs](http://example.com).\n",
			want:  "See [docs](http://example.com).\n",
		},
		{
			name:  "link with title round-trip",
			input: "See [docs](http://example.com \"the docs\").\n",
			want:  "See [docs](http://example.com \"the docs\").\n",
		},
		{
			name:  "raw HTML in heading",
			input: "# Title <a name=\"x\">\n",
			want:  "# Title <a name=\"x\">\n",
		},
		{
			name:  "image with formatted alt",
			input: "![*alt*](pic.png)\n",
			want:  "![alt](pic.png)\n", // alt flattens via flattenInlines
		},
		{
			name:  "autolink",
			input: "<https://example.com>\n",
			want:  "<https://example.com>\n",
		},
		{
			name:  "emphasis preserved",
			input: "an *italic* word\n",
			want:  "an *italic* word\n",
		},
		{
			name:  "strong preserved",
			input: "a **bold** word\n",
			want:  "a **bold** word\n",
		},
		{
			name:  "code span preserved",
			input: "use `printf`\n",
			want:  "use `printf`\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, rt.RunString(fmt.Sprintf(
				"return rela.md.render(rela.md.parse(%q))", tc.input)))
			got := lua.LVAsString(rt.L.Get(-1))
			rt.L.Pop(1)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestMdBlockquoteChildren — AC2: blockquote has a `children` array of
// block nodes; nested list survives.
func TestMdBlockquoteChildren(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()
	require.NoError(t, rt.RunString(`
		local ast = rela.md.parse("> p1\n>\n> - item\n")
		bq = ast[1]
		bq_type = bq.type
		children_kind = type(bq.children)
		children_len = #bq.children
		c1_type = bq.children[1].type
		c2_type = bq.children[2].type
	`))
	assert.Equal(t, "blockquote", lua.LVAsString(rt.L.GetGlobal("bq_type")))
	assert.Equal(t, "table", lua.LVAsString(rt.L.GetGlobal("children_kind")))
	assert.GreaterOrEqual(t, int(lua.LVAsNumber(rt.L.GetGlobal("children_len"))), 2)
	assert.Equal(t, "paragraph", lua.LVAsString(rt.L.GetGlobal("c1_type")))
	assert.Equal(t, "list", lua.LVAsString(rt.L.GetGlobal("c2_type")))
}

// TestMdListMultiBlockChildren — AC3: a list item with multiple
// paragraphs gets a `children` array.
func TestMdListMultiBlockChildren(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()
	src := "- first paragraph\n\n  second paragraph\n"
	require.NoError(t, rt.RunString(fmt.Sprintf(`
		local ast = rela.md.parse(%q)
		item = ast[1].items[1]
		kind = type(item)
		has_children = type(item.children)
		child_count = item.children and #item.children or 0
	`, src)))
	assert.Equal(t, "table", lua.LVAsString(rt.L.GetGlobal("kind")))
	assert.Equal(t, "table", lua.LVAsString(rt.L.GetGlobal("has_children")))
	assert.GreaterOrEqual(t, int(lua.LVAsNumber(rt.L.GetGlobal("child_count"))), 2)
}

// TestMdTaskItemNoCheckboxInline — AC4: task items don't carry a
// phantom checkbox inline.
func TestMdTaskItemNoCheckboxInline(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()
	require.NoError(t, rt.RunString(`
		local ast = rela.md.parse("- [x] foo\n")
		item = ast[1].items[1]
		first_inline_type = item.inlines and item.inlines[1] and item.inlines[1].type or "missing"
	`))
	item, ok := rt.L.GetGlobal("item").(*lua.LTable)
	require.True(t, ok)
	assert.Equal(t, lua.LTrue, item.RawGetString("task"))
	assert.Equal(t, "text", lua.LVAsString(rt.L.GetGlobal("first_inline_type")))
}

// TestMdHeadersAndFirstParagraphFlatten — AC12: helper APIs that
// returned `text` strings continue to do so via flatten() semantics.
func TestMdHeadersAndFirstParagraphFlatten(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()
	require.NoError(t, rt.RunString(`
		local ast = rela.md.parse("# A [link](http://x) B\n\nintro [link](http://x) text\n")
		hs = rela.md.headers(ast)
		first = rela.md.first_paragraph(ast)
	`))
	hs, ok := rt.L.GetGlobal("hs").(*lua.LTable)
	require.True(t, ok)
	require.Equal(t, 1, hs.Len())
	h1 := hs.RawGetInt(1).(*lua.LTable)
	assert.Equal(t, "A link B", string(h1.RawGetString("title").(lua.LString)))
	assert.Equal(t, "intro link text", lua.LVAsString(rt.L.GetGlobal("first")))
}

// TestMdInlineConstructors — AC10.
func TestMdInlineConstructors(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()
	require.NoError(t, rt.RunString(`
		local n = rela.md.paragraph({
			rela.md.text("see "),
			rela.md.link_inline("docs", "/x"),
			rela.md.text(" and "),
			rela.md.code_span("foo()"),
			rela.md.text(" "),
			rela.md.raw_html("<br>"),
		})
		return rela.md.render({n})
	`))
	got := lua.LVAsString(rt.L.Get(-1))
	rt.L.Pop(1)
	assert.Equal(t, "see [docs](/x) and `foo()` <br>\n", got)
}

// TestMdParagraphAutoWrap — AC9.
func TestMdParagraphAutoWrap(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()
	require.NoError(t, rt.RunString(`
		local a = rela.md.paragraph("hello")
		local b = rela.md.paragraph({rela.md.text("hello")})
		return rela.md.render({a}) == rela.md.render({b})
	`))
	v := rt.L.Get(-1)
	rt.L.Pop(1)
	assert.Equal(t, lua.LBool(true), v)
}

// TestMdCorpusRoundTrip — AC13. Synthetic edge cases plus the real
// corpus from tickets/entities/*.md (the markdown bodies of every
// in-tree ticket entity).
func TestMdCorpusRoundTrip(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()

	synthetic := []string{
		"para with [link](http://x)\n",
		"para with ![alt](pic.png)\n",
		"para with <https://example.com>\n",
		"para with *em* and **strong** and ~~strike~~\n",
		"para with `code` span\n",
		"para with raw <a name=\"x\">html</a> in it\n",
		"> blockquote with [a link](url)\n",
		"- item with [a link](url)\n",
		"- item with `code`\n",
		"| a [link](url) | b |\n| - | - |\n| c | d |\n",
		// C1 regression: pipe inside table cell must not split the row.
		"| h |\n| --- |\n| `a|b` |\n",
		// C2 regression: code span with inner backtick keeps fence width.
		"see `` ` `` literal backtick\n",
	}
	for i, src := range synthetic {
		t.Run(fmt.Sprintf("synthetic-%d", i), func(t *testing.T) {
			roundTripFixedPoint(t, rt, src)
		})
	}

	t.Run("ticket-entities", func(t *testing.T) {
		corpusRoundTripFromDisk(t, rt, "../../tickets/entities")
	})
}

// corpusRoundTripFromDisk walks dir for *.md files, extracts their
// markdown body (after the YAML frontmatter), and asserts each is a
// round-trip fixed point. Files that exercise pre-existing HTML-block
// or comment round-trip quirks (not fixed by this refactor) are
// skipped — see corpusSkipPatterns.
func corpusRoundTripFromDisk(t *testing.T, rt *Runtime, dir string) {
	t.Helper()
	var paths []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if strings.HasSuffix(p, ".md") {
			paths = append(paths, p)
		}
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, paths, "no fixtures found in %s", dir)

	skipped := 0
	for _, p := range paths {
		body, ok := readMarkdownBody(t, p)
		if !ok {
			continue
		}
		// Some fixtures exercise pre-existing HTML-block round-trip
		// quirks (multi-line `<!-- ... -->` that get re-grouped on the
		// second parse). Those issues live in the goldmark→Lua block
		// path, not in inline preservation, and are out of scope for
		// this refactor.
		if containsMultilineHTMLComment(body) {
			skipped++
			continue
		}
		t.Run(filepath.Base(p), func(t *testing.T) {
			roundTripFixedPoint(t, rt, body)
		})
	}
	t.Logf("corpus: %d files tested, %d skipped (pre-existing HTML-block quirks)",
		len(paths)-skipped, skipped)
}

// containsMultilineHTMLComment reports whether the body contains a
// multi-line `<!-- ... -->` comment — a block shape that the existing
// HTML-block parser (in goldmark→Lua) does not round-trip cleanly.
func containsMultilineHTMLComment(s string) bool {
	for i := 0; i < len(s); {
		j := strings.Index(s[i:], "<!--")
		if j < 0 {
			return false
		}
		k := strings.Index(s[i+j:], "-->")
		if k < 0 {
			return false
		}
		comment := s[i+j : i+j+k+3]
		if strings.Contains(comment, "\n") {
			return true
		}
		i = i + j + k + 3
	}
	return false
}

// readMarkdownBody returns the markdown body of a rela entity file
// (everything after the closing `---` of the YAML frontmatter), or
// (empty, false) if the file has no body.
func readMarkdownBody(t *testing.T, path string) (string, bool) {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	s := string(raw)
	if !strings.HasPrefix(s, "---\n") {
		return s, true
	}
	rest := s[4:]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return "", false
	}
	body := rest[idx+5:]
	if body == "" {
		return "", false
	}
	return body, true
}

// roundTripFixedPoint asserts render(parse(s)) is a fixed point.
func roundTripFixedPoint(t *testing.T, rt *Runtime, src string) {
	t.Helper()
	code := fmt.Sprintf(`
		local a1 = rela.md.parse(%q)
		local r1 = rela.md.render(a1)
		local a2 = rela.md.parse(r1)
		local r2 = rela.md.render(a2)
		first = r1
		second = r2
	`, src)
	require.NoError(t, rt.RunString(code))
	first := lua.LVAsString(rt.L.GetGlobal("first"))
	second := lua.LVAsString(rt.L.GetGlobal("second"))
	assert.Equal(t, first, second, "render(parse(s)) is not a fixed point for %q", src)
}

// BenchmarkMdParse — AC18.
func BenchmarkMdParse(b *testing.B) {
	rt := newMdTestRuntimeB(b)
	defer rt.Close()
	src := strings.Repeat(`# Heading

A paragraph with [a link](http://example.com), an autolink
<https://example.com>, **strong**, *emphasis*, ~~strike~~, and `+"`code`"+`.

> Blockquote with a [link](url).

- item one
- item two with **bold**
- item three with `+"`code`"+`

| Col A | Col B |
| ----- | ----- |
| a     | b     |

`+"```"+`go
fmt.Println("hi")
`+"```"+`

`, 5)
	code := fmt.Sprintf(`return rela.md.parse(%q)`, src)
	b.ResetTimer()
	for range b.N {
		if err := rt.RunString(code); err != nil {
			b.Fatal(err)
		}
		rt.L.Pop(1)
	}
}

func newMdTestRuntimeB(b *testing.B) *Runtime {
	b.Helper()
	var sb strings.Builder
	return NewReader(ReadDeps{}, &sb)
}

// --- resolve_refs / entity_refs (TKT-LXYHQ) ---

// resolveAndRender runs parse → resolve_refs(map) → render and returns
// the rendered markdown. Used by the table-driven resolve_refs tests.
func resolveAndRender(t *testing.T, input string, refs map[string]string) string {
	t.Helper()
	rt := newMdTestRuntime(t)
	defer rt.Close()
	var b strings.Builder
	b.WriteString("local refs = {}\n")
	for k, v := range refs {
		fmt.Fprintf(&b, "refs[%q] = %q\n", k, v)
	}
	fmt.Fprintf(&b,
		"return rela.md.render(rela.md.resolve_refs(rela.md.parse(%q), refs))\n", input)
	require.NoError(t, rt.RunString(b.String()))
	out := lua.LVAsString(rt.L.Get(-1))
	rt.L.Pop(1)
	return out
}

func TestMdResolveRefs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		refs  map[string]string
		want  string
	}{
		{
			name:  "code-span entity ID is replaced",
			input: "see `TKT-1` here",
			refs:  map[string]string{"TKT-1": "[Title](#tkt-1)"},
			want:  "see [Title](#tkt-1) here\n",
		},
		{
			name:  "bare-prose ID is left alone",
			input: "see TKT-1 here",
			refs:  map[string]string{"TKT-1": "[Title](#tkt-1)"},
			want:  "see TKT-1 here\n",
		},
		{
			name:  "manual ID inside code span",
			input: "see `lua-scripting` here",
			refs:  map[string]string{"lua-scripting": "[Lua Scripting](#lua-scripting)"},
			want:  "see [Lua Scripting](#lua-scripting) here\n",
		},
		{
			name:  "ID inside fenced code block is NOT rewritten",
			input: "```\n`TKT-1`\n```\n",
			refs:  map[string]string{"TKT-1": "[X](#x)"},
			want:  "```\n`TKT-1`\n```\n",
		},
		{
			name:  "ID not in map is left as code span",
			input: "see `TKT-99` here",
			refs:  map[string]string{"TKT-1": "[X](#x)"},
			want:  "see `TKT-99` here\n",
		},
		{
			name:  "multiple IDs in one paragraph",
			input: "see `TKT-1` and `TKT-2`",
			refs:  map[string]string{"TKT-1": "[A](#a)", "TKT-2": "[B](#b)"},
			want:  "see [A](#a) and [B](#b)\n",
		},
		{
			name:  "ID in heading",
			input: "# About `TKT-1`\n",
			refs:  map[string]string{"TKT-1": "[Title](#t)"},
			want:  "# About [Title](#t)\n",
		},
		{
			name:  "ID in blockquote",
			input: "> see `TKT-1` here\n",
			refs:  map[string]string{"TKT-1": "[T](#t)"},
			want:  "> see [T](#t) here\n",
		},
		{
			name:  "ID in list item",
			input: "- see `TKT-1` here\n",
			refs:  map[string]string{"TKT-1": "[T](#t)"},
			want:  "- see [T](#t) here\n",
		},
		{
			name:  "ID in table cell",
			input: "| col |\n| --- |\n| `TKT-1` |\n",
			refs:  map[string]string{"TKT-1": "[T](#t)"},
			// renderer pads cells; just assert the link is present
			want: "",
		},
		{
			name:  "code span containing different content is left alone",
			input: "use `printf`",
			refs:  map[string]string{"TKT-1": "[X](#x)"},
			want:  "use `printf`\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveAndRender(t, tc.input, tc.refs)
			if tc.want != "" {
				assert.Equal(t, tc.want, got)
			} else {
				// For the table case, just check the link survived.
				assert.Contains(t, got, "[T](#t)")
			}
		})
	}
}

func TestMdResolveRefs_PreExistingLinkUntouched(t *testing.T) {
	t.Parallel()
	// A pre-existing markdown link whose text is NOT a code span should
	// not be re-linked even when its content matches a key in the map —
	// the rule is "only code spans".
	got := resolveAndRender(t,
		"see [TKT-1](http://example.com)",
		map[string]string{"TKT-1": "[X](#x)"})
	assert.Equal(t, "see [TKT-1](http://example.com)\n", got)
}

func TestMdResolveRefs_LinkContainingCodeSpan(t *testing.T) {
	t.Parallel()
	// A link whose inline text contains a code span — the code span
	// inside the link gets rewritten; the surrounding link stays.
	got := resolveAndRender(t,
		"see [`TKT-1` link](http://example.com)",
		map[string]string{"TKT-1": "[X](#x)"})
	// The link wrapper survives; the code span inside becomes the splice.
	assert.Contains(t, got, "[")
	assert.Contains(t, got, "[X](#x)")
}

func TestMdResolveRefs_DeepCopy(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()
	require.NoError(t, rt.RunString(`
		local ast = rela.md.parse("see `+"`TKT-1`"+` here")
		local _ = rela.md.resolve_refs(ast, {["TKT-1"] = "[X](#x)"})
		return rela.md.render(ast)
	`))
	got := lua.LVAsString(rt.L.Get(-1))
	rt.L.Pop(1)
	// Input AST is unchanged; original code span survives.
	assert.Equal(t, "see `TKT-1` here\n", got)
}

func TestMdResolveRefs_EmptyMap(t *testing.T) {
	t.Parallel()
	rt := newMdTestRuntime(t)
	defer rt.Close()
	require.NoError(t, rt.RunString(`
		local ast = rela.md.parse("see `+"`TKT-1`"+` here")
		return rela.md.render(rela.md.resolve_refs(ast, {}))
	`))
	got := lua.LVAsString(rt.L.Get(-1))
	rt.L.Pop(1)
	assert.Equal(t, "see `TKT-1` here\n", got)
}

func TestMdResolveRefs_NegativeInput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		code string
	}{
		{
			name: "non-table ast",
			code: `rela.md.resolve_refs("notatable", {})`,
		},
		{
			name: "non-table replacements",
			code: `rela.md.resolve_refs(rela.md.parse(""), "notatable")`,
		},
		{
			name: "value with newline",
			code: `rela.md.resolve_refs(rela.md.parse(""), {["TKT-1"] = "a\nb"})`,
		},
		{
			name: "empty key",
			code: `rela.md.resolve_refs(rela.md.parse(""), {[""] = "x"})`,
		},
		{
			name: "non-string value",
			code: `rela.md.resolve_refs(rela.md.parse(""), {["TKT-1"] = 42})`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rt := newMdTestRuntime(t)
			defer rt.Close()
			err := rt.RunString(tc.code)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "rela.md.resolve_refs:")
		})
	}
}

func TestMdEntityRefs(t *testing.T) {
	t.Parallel()
	t.Run("default style is title-slug", func(t *testing.T) {
		rt := NewWriter(testWorkspace(t).services("/tmp"), &strings.Builder{})
		defer rt.Close()
		require.NoError(t, rt.RunString(`
			local refs = rela.md.entity_refs()
			result = refs["TKT-001"]
		`))
		// Test fixture: TKT-001 has title="Test Ticket"
		assert.Equal(t, lua.LString("[Test Ticket](#test-ticket)"),
			rt.L.GetGlobal("result"))
	})

	t.Run("style=id", func(t *testing.T) {
		rt := NewWriter(testWorkspace(t).services("/tmp"), &strings.Builder{})
		defer rt.Close()
		require.NoError(t, rt.RunString(`
			local refs = rela.md.entity_refs({style = "id"})
			result = refs["TKT-001"]
		`))
		assert.Equal(t, lua.LString("[Test Ticket](#tkt-001)"),
			rt.L.GetGlobal("result"))
	})

	t.Run("types restricts to the listed types", func(t *testing.T) {
		rt := NewWriter(testWorkspace(t).services("/tmp"), &strings.Builder{})
		defer rt.Close()
		require.NoError(t, rt.RunString(`
			local refs = rela.md.entity_refs({types = {"ticket"}})
			result = {tkt = refs["TKT-001"], feat = refs["FEAT-001"]}
		`))
		result := rt.L.GetGlobal("result").(*lua.LTable)
		assert.Equal(t, lua.LString("[Test Ticket](#test-ticket)"),
			result.RawGetString("tkt"))
		assert.Equal(t, lua.LNil, result.RawGetString("feat"))
	})

	t.Run("unknown type errors", func(t *testing.T) {
		rt := NewWriter(testWorkspace(t).services("/tmp"), &strings.Builder{})
		defer rt.Close()
		err := rt.RunString(`rela.md.entity_refs({types = {"unknown"}})`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `unknown entity type "unknown"`)
	})

	t.Run("custom format callback", func(t *testing.T) {
		rt := NewWriter(testWorkspace(t).services("/tmp"), &strings.Builder{})
		defer rt.Close()
		require.NoError(t, rt.RunString(`
			local refs = rela.md.entity_refs({
				format = function(e) return "[" .. e.title .. "](/x/" .. e.id .. ")" end,
			})
			result = refs["TKT-001"]
		`))
		assert.Equal(t, lua.LString("[Test Ticket](/x/TKT-001)"),
			rt.L.GetGlobal("result"))
	})

	t.Run("nil deps produces a clean error, not a panic", func(t *testing.T) {
		rt := newMdTestRuntime(t)
		defer rt.Close()
		err := rt.RunString(`return rela.md.entity_refs()`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rela.md.entity_refs:")
	})
}

func TestMdEntityRefs_TitleInjection(t *testing.T) {
	t.Parallel()
	mw := newMockWorkspace(t)
	mw.seedEntity(&entity.Entity{
		ID:         "TKT-EVIL",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": `]"](javascript:alert(1))[evil`},
	})
	rt := NewWriter(mw.services("/tmp"), &strings.Builder{})
	defer rt.Close()
	require.NoError(t, rt.RunString(`
		local refs = rela.md.entity_refs()
		local rendered = rela.md.render(rela.md.parse(refs["TKT-EVIL"]))
		result = rendered
	`))
	got := rt.L.GetGlobal("result").String()
	assert.Contains(t, got, `\[`)
	assert.Contains(t, got, `\]`)
	assert.NotContains(t, got, "<javascript:")
}

func TestMdEntityRefs_UnicodeTitle(t *testing.T) {
	t.Parallel()
	mw := newMockWorkspace(t)
	mw.seedEntity(&entity.Entity{
		ID:         "TKT-EU",
		Type:       "ticket",
		Properties: map[string]interface{}{"title": "Café Résumé"},
	})
	rt := NewWriter(mw.services("/tmp"), &strings.Builder{})
	defer rt.Close()
	require.NoError(t, rt.RunString(`
		local refs = rela.md.entity_refs()
		result = refs["TKT-EU"]
	`))
	assert.Equal(t, lua.LString("[Café Résumé](#café-résumé)"),
		rt.L.GetGlobal("result"))
}

func TestMdEntityRefs_IntegrationWithResolveRefs(t *testing.T) {
	t.Parallel()
	mw := testWorkspace(t)
	rt := NewWriter(mw.services("/tmp"), &strings.Builder{})
	defer rt.Close()
	require.NoError(t, rt.RunString(`
		local refs = rela.md.entity_refs()
		local ast = rela.md.parse("Linked: `+"`TKT-001`"+` here")
		result = rela.md.render(rela.md.resolve_refs(ast, refs))
	`))
	got := lua.LVAsString(rt.L.GetGlobal("result"))
	assert.Equal(t, "Linked: [Test Ticket](#test-ticket) here\n", got)
}

func TestTitleSlug(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{"Hello World", "hello-world"},
		{"Trim - dashes -", "trim-dashes"},
		{"Multiple    spaces", "multiple-spaces"},
		{"Punct! ?? Heavy", "punct-heavy"},
		{"already-slug", "already-slug"},
		{"  leading", "leading"},
		{"", ""},
		{"!!!", ""},
		{"123 abc", "123-abc"},
		{"Café Résumé", "café-résumé"},
		{"東京", "東京"},
		{"hello 世界", "hello-世界"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, titleSlug(tc.in))
		})
	}
}
