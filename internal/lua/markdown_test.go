package lua

import (
	"fmt"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMdTestRuntime creates a minimal runtime for markdown tests (no workspace needed).
func newMdTestRuntime(t *testing.T) *Runtime {
	t.Helper()
	var sb strings.Builder
	return NewWriter(WriteDeps{}, &sb)
}

func TestMdParse(t *testing.T) {
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
	assert.Equal(t, "done", string(item1.RawGetString("text").(lua.LString)))

	item2, ok := rt.L.GetGlobal("item2").(*lua.LTable)
	require.True(t, ok, "item2 should be a table")
	assert.Equal(t, lua.LTrue, item2.RawGetString("task"))
	assert.Equal(t, lua.LFalse, item2.RawGetString("checked"))
	assert.Equal(t, "todo", string(item2.RawGetString("text").(lua.LString)))
}

func TestMdTaskListRender(t *testing.T) {
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
	rt := newMdTestRuntime(t)
	defer rt.Close()

	// Task first, then plain.
	code := `
		local ast = rela.md.parse("- [x] task\n- plain\n")
		count = #ast[1].items
		item1_type = type(ast[1].items[1])
		item1_task = type(ast[1].items[1]) == "table" and ast[1].items[1].task or false
		item1_text = type(ast[1].items[1]) == "table" and ast[1].items[1].text or ast[1].items[1]
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
			extract: "ast[1].text",
			want:    "This is ~~struck~~ text.",
		},
		{
			name:    "strikethrough in heading",
			input:   "# Title with ~~struck~~ word\n",
			extract: "ast[1].text",
			want:    "Title with ~~struck~~ word",
		},
		{
			name:    "strikethrough in blockquote",
			input:   "> quoted ~~struck~~ text\n",
			extract: "ast[1].content",
			want:    "quoted ~~struck~~ text",
		},
		{
			name:    "strikethrough in table cell",
			input:   "| h |\n|---|\n| ~~struck~~ |\n",
			extract: "ast[1].rows[1][1]",
			want:    "~~struck~~",
		},
		{
			name:    "strikethrough in task item",
			input:   "- [x] foo ~~bar~~ baz\n",
			extract: "ast[1].items[1].text",
			want:    "foo ~~bar~~ baz",
		},
		// Code spans are preserved.
		{
			name:    "code span in paragraph",
			input:   "Use `printf` for output.\n",
			extract: "ast[1].text",
			want:    "Use `printf` for output.",
		},
		{
			name:    "code span in task item",
			input:   "- [x] call `foo()`\n",
			extract: "ast[1].items[1].text",
			want:    "call `foo()`",
		},
		// Strikethrough does NOT activate inside fenced code blocks.
		{
			name:    "code block content keeps tildes literally",
			input:   "```\n~~not struck~~\n```\n",
			extract: "ast[1].content",
			want:    "~~not struck~~",
		},
		// Bold/italic/links are intentionally dropped per policy.
		{
			name:    "bold dropped in task item",
			input:   "- [x] **bold** text\n",
			extract: "ast[1].items[1].text",
			want:    "bold text",
		},
		{
			name:    "italic dropped in paragraph",
			input:   "Some *italic* word.\n",
			extract: "ast[1].text",
			want:    "Some italic word.",
		},
		{
			name:    "link dropped to text only",
			input:   "See [docs](http://example.com).\n",
			extract: "ast[1].text",
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
	rt := newMdTestRuntime(t)
	defer rt.Close()

	code := `
		local ast = rela.md.parse("- [x] \n- [ ] \n")
		count = #ast[1].items
		item1_task = ast[1].items[1].task
		item1_checked = ast[1].items[1].checked
		item1_text = ast[1].items[1].text
		item2_task = ast[1].items[2].task
		item2_checked = ast[1].items[2].checked
		item2_text = ast[1].items[2].text
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
	rt := newMdTestRuntime(t)
	defer rt.Close()

	// A list item with a continuation line — single TextBlock, captured fully.
	code := `
		local ast = rela.md.parse("- [x] first line\n  second line\n")
		result = ast[1].items[1].text
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
	rt := newMdTestRuntime(t)
	defer rt.Close()

	t.Run("simple table", func(t *testing.T) {
		code := `
			local ast = rela.md.parse("| Name | Age |\n| ---- | --- |\n| Alice | 30 |\n| Bob | 25 |\n")
			result_len = #ast
			result_type = ast[1].type
			result_header_len = #ast[1].header
			result_h1 = ast[1].header[1]
			result_h2 = ast[1].header[2]
			result_rows_len = #ast[1].rows
			result_r1c1 = ast[1].rows[1][1]
			result_r1c2 = ast[1].rows[1][2]
			result_r2c1 = ast[1].rows[2][1]
			result_r2c2 = ast[1].rows[2][2]
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
			result_cell = ast[1].rows[1][1]
		`
		require.NoError(t, rt.RunString(code))

		assert.Equal(t, "bold text", lua.LVAsString(rt.L.GetGlobal("result_cell")))
	})
}

func TestMdTableRender(t *testing.T) {
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
