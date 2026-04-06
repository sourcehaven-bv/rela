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
	return New(nil, nil, "", &sb)
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
		assert.Equal(t, "", result)
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
	rt := New(ws, testMeta(), "/project", &sb)
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
