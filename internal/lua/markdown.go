// Package lua provides markdown AST manipulation functions for Lua scripts.
// The rela.md module enables parsing, transforming, and rendering markdown content.
package lua

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// blockKind tags a block-level AST node by its `type` field.
type blockKind string

// inlineKind tags an inline AST node by its `type` field.
type inlineKind string

// Block node types.
const (
	nodeTypeHeading       blockKind = "heading"
	nodeTypeParagraph     blockKind = "paragraph"
	nodeTypeCodeBlock     blockKind = "code_block"
	nodeTypeList          blockKind = "list"
	nodeTypeBlockquote    blockKind = "blockquote"
	nodeTypeThematicBreak blockKind = "thematic_break"
	nodeTypeTable         blockKind = "table"
	nodeTypeRaw           blockKind = "raw"
)

// Inline node types. Block leaves (paragraph, heading) and
// inline-bearing containers (link, emphasis, strong, strikethrough,
// image alt) carry arrays of these.
const (
	inlineTypeText          inlineKind = "text"
	inlineTypeCodeSpan      inlineKind = "code_span"
	inlineTypeRawHTML       inlineKind = "raw_html"
	inlineTypeAutolink      inlineKind = "autolink"
	inlineTypeSoftBreak     inlineKind = "soft_break"
	inlineTypeHardBreak     inlineKind = "hard_break"
	inlineTypeEmphasis      inlineKind = "emphasis"
	inlineTypeStrong        inlineKind = "strong"
	inlineTypeStrikethrough inlineKind = "strikethrough"
	inlineTypeLink          inlineKind = "link"
	inlineTypeImage         inlineKind = "image"
)

const (
	alignLeft   = "left"
	alignRight  = "right"
	alignCenter = "center"
	alignNone   = "none"

	minHeaderLevel = 1
	maxHeaderLevel = 6
)

// registerMarkdownModule adds the rela.md submodule to the rela table.
// Note: The Lua VM (LState) is not thread-safe. Each goroutine must use
// its own Runtime instance.
func (r *Runtime) registerMarkdownModule(rela *lua.LTable) {
	md := r.L.NewTable()

	// Core functions
	r.L.SetField(md, "parse", r.L.NewFunction(r.luaMdParse))
	r.L.SetField(md, "render", r.L.NewFunction(r.luaMdRender))

	// Transformation functions
	r.L.SetField(md, "shift_headers", r.L.NewFunction(r.luaMdShiftHeaders))
	r.L.SetField(md, "set_min_header_level", r.L.NewFunction(r.luaMdSetMinHeaderLevel))

	// Extraction functions
	r.L.SetField(md, "headers", r.L.NewFunction(r.luaMdHeaders))
	r.L.SetField(md, "extract_section", r.L.NewFunction(r.luaMdExtractSection))
	r.L.SetField(md, "first_paragraph", r.L.NewFunction(r.luaMdFirstParagraph))

	// Composition functions
	r.L.SetField(md, "concat", r.L.NewFunction(r.luaMdConcat))

	// Node constructors (block).
	r.L.SetField(md, "heading", r.L.NewFunction(luaMdHeading))
	r.L.SetField(md, "paragraph", r.L.NewFunction(luaMdParagraph))
	r.L.SetField(md, "code_block", r.L.NewFunction(luaMdCodeBlock))
	r.L.SetField(md, "thematic_break", r.L.NewFunction(luaMdThematicBreak))
	r.L.SetField(md, "blockquote", r.L.NewFunction(luaMdBlockquote))
	r.L.SetField(md, "list", r.L.NewFunction(luaMdList))

	// Node constructors (inline).
	r.L.SetField(md, "text", r.L.NewFunction(luaMdInlineText))
	r.L.SetField(md, "code_span", r.L.NewFunction(luaMdInlineCodeSpan))
	r.L.SetField(md, "link_inline", r.L.NewFunction(luaMdInlineLink))
	r.L.SetField(md, "raw_html", r.L.NewFunction(luaMdInlineRawHTML))

	// Inline-flatten helper for scripts.
	r.L.SetField(md, "flatten", r.L.NewFunction(luaMdFlatten))

	// Generation helpers (string-returning, predate the inline AST).
	r.L.SetField(md, "link", r.L.NewFunction(luaMdLink))
	r.L.SetField(md, "ref", r.L.NewFunction(luaMdRef))
	r.L.SetField(md, "table", r.L.NewFunction(luaMdTable))
	r.L.SetField(md, "entity_table", r.L.NewFunction(r.luaMdEntityTable))

	// Reference resolution: rewrite code-span entity-ID tokens to links.
	r.L.SetField(md, "resolve_refs", r.L.NewFunction(r.luaMdResolveRefs))
	r.L.SetField(md, "entity_refs", r.L.NewFunction(r.luaMdEntityRefs))

	r.L.SetField(rela, "md", md)
}

// --- Inline node constructors ---

// luaMdInlineText: rela.md.text(s) -> {type="text", text=s}
func luaMdInlineText(ls *lua.LState) int {
	s := ls.CheckString(1)
	t := ls.NewTable()
	t.RawSetString("type", lua.LString(inlineTypeText))
	t.RawSetString("text", lua.LString(s))
	ls.Push(t)
	return 1
}

// luaMdInlineCodeSpan: rela.md.code_span(s) -> {type="code_span", text=s}
func luaMdInlineCodeSpan(ls *lua.LState) int {
	s := ls.CheckString(1)
	t := ls.NewTable()
	t.RawSetString("type", lua.LString(inlineTypeCodeSpan))
	t.RawSetString("text", lua.LString(s))
	ls.Push(t)
	return 1
}

// luaMdInlineRawHTML: rela.md.raw_html(s) -> {type="raw_html", text=s}
func luaMdInlineRawHTML(ls *lua.LState) int {
	s := ls.CheckString(1)
	t := ls.NewTable()
	t.RawSetString("type", lua.LString(inlineTypeRawHTML))
	t.RawSetString("text", lua.LString(s))
	ls.Push(t)
	return 1
}

// luaMdInlineLink: rela.md.link_inline(text_or_inlines, url, title?) -> link inline.
// Distinct from rela.md.link, which returns a string [text](url) (predates
// the inline AST).
func luaMdInlineLink(ls *lua.LState) int {
	inlines := acceptInlinesArg(ls, 1)
	url := ls.CheckString(2)
	title := ls.OptString(3, "")

	t := ls.NewTable()
	t.RawSetString("type", lua.LString(inlineTypeLink))
	t.RawSetString("url", lua.LString(url))
	if title != "" {
		t.RawSetString("title", lua.LString(title))
	}
	t.RawSetString("inlines", inlines)
	ls.Push(t)
	return 1
}

// luaMdFlatten: rela.md.flatten(inlines) -> string
// Applies the legacy text-extraction policy to a Lua-side inlines array.
func luaMdFlatten(ls *lua.LState) int {
	v := ls.Get(1)
	tbl, ok := v.(*lua.LTable)
	if !ok {
		ls.RaiseError("rela.md.flatten: expected an inlines table")
		return 0
	}
	ls.Push(lua.LString(flattenInlines(tbl)))
	return 1
}

// --- Core Functions ---

// luaMdParse parses markdown content into an AST table.
// Usage: local ast = rela.md.parse(content)
func (r *Runtime) luaMdParse(ls *lua.LState) int {
	content := ls.CheckString(1)

	source := []byte(content)
	md := goldmark.New(goldmark.WithExtensions(
		extension.NewTable(),
		extension.TaskList,
		extension.Strikethrough,
	))
	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)

	astTable := r.goldmarkToLua(doc, source)
	ls.Push(astTable)
	return 1
}

// luaMdRender renders an AST table back to markdown string.
// Usage: local md = rela.md.render(ast)
func (r *Runtime) luaMdRender(ls *lua.LState) int {
	astTable := ls.CheckTable(1)

	var sb strings.Builder
	renderNodes(&sb, astTable)

	ls.Push(lua.LString(sb.String()))
	return 1
}

// --- Transformation Functions ---

// luaMdShiftHeaders shifts all header levels by the given offset.
// Usage: ast = rela.md.shift_headers(ast, 1)  -- # becomes ##
func (r *Runtime) luaMdShiftHeaders(ls *lua.LState) int {
	astTable := ls.CheckTable(1)
	offset := ls.CheckInt(2)

	result := r.L.NewTable()

	// Use sequential access to preserve document order
	for i := 1; i <= astTable.Len(); i++ {
		v := astTable.RawGetInt(i)
		node, ok := v.(*lua.LTable)
		if !ok {
			continue
		}
		newNode := r.shiftNodeHeaders(node, offset)
		result.RawSetInt(i, newNode)
	}

	ls.Push(result)
	return 1
}

// luaMdSetMinHeaderLevel normalizes headers so minimum level equals target.
// Usage: ast = rela.md.set_min_header_level(ast, 2)  -- min becomes ##
func (r *Runtime) luaMdSetMinHeaderLevel(ls *lua.LState) int {
	astTable := ls.CheckTable(1)
	targetLevel := ls.CheckInt(2)

	// Clamp target level to valid range
	if targetLevel < minHeaderLevel {
		targetLevel = minHeaderLevel
	}
	if targetLevel > maxHeaderLevel {
		targetLevel = maxHeaderLevel
	}

	// Find current minimum level
	minLevel := r.findMinHeaderLevel(astTable)

	// No headers found or already at target, return as-is
	if minLevel > maxHeaderLevel {
		ls.Push(astTable)
		return 1
	}

	offset := targetLevel - minLevel
	if offset == 0 {
		ls.Push(astTable)
		return 1
	}

	// Apply shift using sequential access to preserve document order
	result := r.L.NewTable()
	for i := 1; i <= astTable.Len(); i++ {
		v := astTable.RawGetInt(i)
		node, ok := v.(*lua.LTable)
		if !ok {
			continue
		}
		newNode := r.shiftNodeHeaders(node, offset)
		result.RawSetInt(i, newNode)
	}

	ls.Push(result)
	return 1
}

// findMinHeaderLevel finds the minimum header level in the AST.
func (r *Runtime) findMinHeaderLevel(astTable *lua.LTable) int {
	minLevel := maxHeaderLevel + 1
	// Order doesn't matter for finding minimum, but use sequential for consistency
	for i := 1; i <= astTable.Len(); i++ {
		v := astTable.RawGetInt(i)
		node, ok := v.(*lua.LTable)
		if !ok {
			continue
		}
		if nodeType := node.RawGetString("type"); nodeType != lua.LString(nodeTypeHeading) {
			continue
		}
		level, ok := node.RawGetString("level").(lua.LNumber)
		if !ok {
			continue
		}
		if int(level) < minLevel {
			minLevel = int(level)
		}
	}
	return minLevel
}

// --- Extraction Functions ---

// luaMdHeaders extracts a list of headers from the AST.
// Usage: local headers = rela.md.headers(ast)
// Usage: local headers = rela.md.headers(ast, {min_level=2, max_level=3})
func (r *Runtime) luaMdHeaders(ls *lua.LState) int {
	astTable := ls.CheckTable(1)

	minLvl, maxLvl := r.parseHeaderOptions(ls)

	result := r.L.NewTable()
	resultIdx := 1

	// Use sequential access to preserve document order
	for i := 1; i <= astTable.Len(); i++ {
		v := astTable.RawGetInt(i)
		node, ok := v.(*lua.LTable)
		if !ok {
			continue
		}
		if nodeType := node.RawGetString("type"); nodeType != lua.LString(nodeTypeHeading) {
			continue
		}
		level := 1
		if l, ok := node.RawGetString("level").(lua.LNumber); ok {
			level = int(l)
		}

		if level >= minLvl && level <= maxLvl {
			header := r.L.NewTable()
			header.RawSetString("level", lua.LNumber(level))
			header.RawSetString("title", lua.LString(headingTitleFlat(node)))
			result.RawSetInt(resultIdx, header)
			resultIdx++
		}
	}

	ls.Push(result)
	return 1
}

// parseHeaderOptions extracts min_level and max_level from options table.
func (r *Runtime) parseHeaderOptions(ls *lua.LState) (minLvl, maxLvl int) {
	minLvl = minHeaderLevel
	maxLvl = maxHeaderLevel

	if ls.GetTop() < 2 || ls.Get(2).Type() != lua.LTTable {
		return
	}

	opts := ls.CheckTable(2)
	if v := opts.RawGetString("min_level"); v != lua.LNil {
		if n, ok := v.(lua.LNumber); ok {
			minLvl = int(n)
		}
	}
	if v := opts.RawGetString("max_level"); v != lua.LNil {
		if n, ok := v.(lua.LNumber); ok {
			maxLvl = int(n)
		}
	}
	return
}

// luaMdExtractSection extracts nodes under a matching header until next same-level header.
// Only extracts the first matching section.
// Usage: local section = rela.md.extract_section(ast, "Overview")
func (r *Runtime) luaMdExtractSection(ls *lua.LState) int {
	astTable := ls.CheckTable(1)
	pattern := ls.CheckString(2)

	result := r.L.NewTable()
	resultIdx := 1
	capturing := false
	done := false
	captureLevel := 0

	// Use sequential access to preserve document order
	for i := 1; i <= astTable.Len() && !done; i++ {
		v := astTable.RawGetInt(i)
		node, ok := v.(*lua.LTable)
		if !ok {
			continue
		}

		nodeType, _ := node.RawGetString("type").(lua.LString)

		if nodeType == lua.LString(nodeTypeHeading) {
			level, title := r.getHeadingInfo(node)

			if capturing && level <= captureLevel {
				// Stop if we hit a header at same or higher level
				done = true
				continue
			}
			if !capturing && strings.Contains(title, pattern) {
				// Start capturing if header matches pattern
				capturing = true
				captureLevel = level
			}
		}

		if capturing {
			result.RawSetInt(resultIdx, r.deepCopyNode(node))
			resultIdx++
		}
	}

	if result.Len() == 0 {
		ls.Push(lua.LNil)
	} else {
		ls.Push(result)
	}
	return 1
}

// getHeadingInfo extracts level and a flat title from a heading node.
func (r *Runtime) getHeadingInfo(node *lua.LTable) (level int, title string) {
	level = 1
	if l, ok := node.RawGetString("level").(lua.LNumber); ok {
		level = int(l)
	}
	title = headingTitleFlat(node)
	return
}

// headingTitleFlat reads the heading's flat title using the legacy
// flatten policy (drops link wrappers, drops emphasis, preserves `~~`
// and code spans). Falls back to a `text` field for hand-built nodes.
func headingTitleFlat(node *lua.LTable) string {
	if t := inlinesField(node, "inlines"); t != nil {
		return flattenInlines(t)
	}
	return stringField(node, "text")
}

// luaMdFirstParagraph extracts the first paragraph text from the AST.
// Usage: local text = rela.md.first_paragraph(ast)
func (r *Runtime) luaMdFirstParagraph(ls *lua.LState) int {
	astTable := ls.CheckTable(1)

	// Use sequential access to preserve document order
	for i := 1; i <= astTable.Len(); i++ {
		v := astTable.RawGetInt(i)
		node, ok := v.(*lua.LTable)
		if !ok {
			continue
		}
		if blockKindOf(node) == nodeTypeParagraph {
			if inlines := inlinesField(node, "inlines"); inlines != nil {
				ls.Push(lua.LString(flattenInlines(inlines)))
			} else {
				// Fallback for hand-built paragraphs carrying `text`.
				ls.Push(lua.LString(stringField(node, "text")))
			}
			return 1
		}
	}

	ls.Push(lua.LNil)
	return 1
}

// --- Composition Functions ---

// luaMdConcat concatenates multiple ASTs into one.
// Usage: local combined = rela.md.concat(ast1, ast2, ast3)
func (r *Runtime) luaMdConcat(ls *lua.LState) int {
	result := r.L.NewTable()
	resultIdx := 1

	// Process each argument using sequential access to preserve order
	for i := 1; i <= ls.GetTop(); i++ {
		arg := ls.Get(i)
		tbl, ok := arg.(*lua.LTable)
		if !ok {
			continue
		}
		for j := 1; j <= tbl.Len(); j++ {
			v := tbl.RawGetInt(j)
			node, ok := v.(*lua.LTable)
			if !ok {
				continue
			}
			result.RawSetInt(resultIdx, r.deepCopyNode(node))
			resultIdx++
		}
	}

	ls.Push(result)
	return 1
}

// --- Node Constructors ---

// luaMdHeading creates a heading node.
// Usage: local node = rela.md.heading(2, "Section Title")
//
//	local node = rela.md.heading(2, {rela.md.text("a "), rela.md.code_span("b")})
func luaMdHeading(ls *lua.LState) int {
	level := ls.CheckInt(1)
	if level < minHeaderLevel {
		level = minHeaderLevel
	}
	if level > maxHeaderLevel {
		level = maxHeaderLevel
	}
	inlines := acceptInlinesArg(ls, 2)

	node := ls.NewTable()
	node.RawSetString("type", lua.LString(nodeTypeHeading))
	node.RawSetString("level", lua.LNumber(level))
	node.RawSetString("inlines", inlines)

	ls.Push(node)
	return 1
}

// luaMdParagraph creates a paragraph node.
// Usage: local node = rela.md.paragraph("Some text content")
//
//	local node = rela.md.paragraph({rela.md.text("see "), rela.md.link_inline("link", "/x")})
func luaMdParagraph(ls *lua.LState) int {
	inlines := acceptInlinesArg(ls, 1)

	node := ls.NewTable()
	node.RawSetString("type", lua.LString(nodeTypeParagraph))
	node.RawSetString("inlines", inlines)

	ls.Push(node)
	return 1
}

// acceptInlinesArg coerces argument idx to an inlines array. Accepts:
//   - a string: wraps as {{type="text", text=s}}
//   - a table: passes through verbatim (assumed to be an inlines array)
func acceptInlinesArg(ls *lua.LState, idx int) *lua.LTable {
	v := ls.Get(idx)
	switch x := v.(type) {
	case lua.LString:
		out := ls.NewTable()
		if string(x) != "" {
			leaf := ls.NewTable()
			leaf.RawSetString("type", lua.LString(inlineTypeText))
			leaf.RawSetString("text", x)
			out.RawSetInt(1, leaf)
		}
		return out
	case *lua.LTable:
		return x
	}
	ls.RaiseError("expected string or inlines table at argument %d", idx)
	return nil
}

// luaMdCodeBlock creates a code block node.
// Usage: local node = rela.md.code_block("print('hello')", "lua")
func luaMdCodeBlock(ls *lua.LState) int {
	content := ls.CheckString(1)
	language := ls.OptString(2, "")

	node := ls.NewTable()
	node.RawSetString("type", lua.LString(nodeTypeCodeBlock))
	node.RawSetString("content", lua.LString(content))
	node.RawSetString("language", lua.LString(language))

	ls.Push(node)
	return 1
}

// luaMdThematicBreak creates a thematic break (horizontal rule) node.
// Usage: local node = rela.md.thematic_break()
func luaMdThematicBreak(ls *lua.LState) int {
	node := ls.NewTable()
	node.RawSetString("type", lua.LString(nodeTypeThematicBreak))

	ls.Push(node)
	return 1
}

// luaMdBlockquote creates a blockquote node.
// Usage: local node = rela.md.blockquote("Quoted text")
//
//	local node = rela.md.blockquote({rela.md.paragraph("p1"), rela.md.paragraph("p2")})
func luaMdBlockquote(ls *lua.LState) int {
	v := ls.Get(1)
	node := ls.NewTable()
	node.RawSetString("type", lua.LString(nodeTypeBlockquote))

	switch x := v.(type) {
	case lua.LString:
		// String → wrap as a single paragraph child.
		children := ls.NewTable()
		para := ls.NewTable()
		para.RawSetString("type", lua.LString(nodeTypeParagraph))
		inlines := ls.NewTable()
		if string(x) != "" {
			leaf := ls.NewTable()
			leaf.RawSetString("type", lua.LString(inlineTypeText))
			leaf.RawSetString("text", x)
			inlines.RawSetInt(1, leaf)
		}
		para.RawSetString("inlines", inlines)
		children.RawSetInt(1, para)
		node.RawSetString("children", children)
	case *lua.LTable:
		// Table → assumed to be a children array of block nodes.
		node.RawSetString("children", x)
	default:
		ls.RaiseError("rela.md.blockquote: expected string or children table")
		return 0
	}

	ls.Push(node)
	return 1
}

// luaMdList creates a list node.
//
// Items may be plain strings or tables. A table item with task=true (an
// explicit lua boolean true) becomes a task list checkbox; the table is
// expected to have:
//
//	{task=true, checked=<bool>, text=<string>}
//
// Plain table items (without task=true) render using the text field as a
// regular bullet item. Missing text renders as empty string.
//
// Usage:
//
//	rela.md.list({"item1", "item2"})                       -- plain
//	rela.md.list({"item1", "item2"}, true)                 -- ordered
//	rela.md.list({                                         -- task list
//	    {task=true, checked=true,  text="done"},
//	    {task=true, checked=false, text="todo"},
//	})
func luaMdList(ls *lua.LState) int {
	itemsTable := ls.CheckTable(1)
	ordered := ls.OptBool(2, false)

	items := ls.NewTable()
	// Use sequential access to preserve item order
	for i := 1; i <= itemsTable.Len(); i++ {
		v := itemsTable.RawGetInt(i)
		switch item := v.(type) {
		case lua.LString:
			items.RawSetInt(i, item)
		case *lua.LTable:
			// Nested list or complex item
			items.RawSetInt(i, item)
		}
	}

	node := ls.NewTable()
	node.RawSetString("type", lua.LString(nodeTypeList))
	node.RawSetString("ordered", lua.LBool(ordered))
	node.RawSetString("items", items)

	ls.Push(node)
	return 1
}

// --- Helper Functions ---

// goldmarkToLua converts a goldmark AST document to a Lua table.
func (r *Runtime) goldmarkToLua(doc ast.Node, source []byte) *lua.LTable {
	result := r.L.NewTable()
	idx := 1

	// Walk only top-level children of the document
	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		if node := r.nodeToLua(child, source); node != nil {
			result.RawSetInt(idx, node)
			idx++
		}
	}

	return result
}

// nodeToLua converts a single goldmark AST node to a Lua table.
func (r *Runtime) nodeToLua(n ast.Node, source []byte) *lua.LTable {
	node := r.L.NewTable()

	switch n := n.(type) {
	case *ast.Heading:
		node.RawSetString("type", lua.LString(nodeTypeHeading))
		node.RawSetString("level", lua.LNumber(n.Level))
		node.RawSetString("inlines", r.extractInlines(n, source))

	case *ast.Paragraph:
		node.RawSetString("type", lua.LString(nodeTypeParagraph))
		node.RawSetString("inlines", r.extractInlines(n, source))

	case *ast.TextBlock:
		// goldmark emits TextBlock for paragraph-equivalent content inside
		// list items and other contexts. Treat as a paragraph for our
		// purposes so block-level walkers don't have to special-case it.
		node.RawSetString("type", lua.LString(nodeTypeParagraph))
		node.RawSetString("inlines", r.extractInlines(n, source))

	case *ast.FencedCodeBlock:
		node.RawSetString("type", lua.LString(nodeTypeCodeBlock))
		node.RawSetString("language", lua.LString(string(n.Language(source))))
		node.RawSetString("content", lua.LString(r.extractCodeBlockContent(n, source)))

	case *ast.CodeBlock:
		node.RawSetString("type", lua.LString(nodeTypeCodeBlock))
		node.RawSetString("language", lua.LString(""))
		node.RawSetString("content", lua.LString(r.extractLinesContent(n, source)))

	case *ast.List:
		node.RawSetString("type", lua.LString(nodeTypeList))
		node.RawSetString("ordered", lua.LBool(n.IsOrdered()))
		node.RawSetString("items", r.extractListItems(n, source))

	case *ast.Blockquote:
		node.RawSetString("type", lua.LString(nodeTypeBlockquote))
		node.RawSetString("children", r.extractBlockChildren(n, source))

	case *ast.ThematicBreak:
		node.RawSetString("type", lua.LString(nodeTypeThematicBreak))

	case *east.Table:
		node.RawSetString("type", lua.LString(nodeTypeTable))
		header, rows, alignments := r.extractTableData(n, source)
		node.RawSetString("header", header)
		node.RawSetString("rows", rows)
		node.RawSetString("alignments", alignments)

	default:
		// For unsupported node types, capture as raw
		node.RawSetString("type", lua.LString(nodeTypeRaw))
		node.RawSetString("content", lua.LString(r.extractRawContent(n, source)))
	}

	return node
}

// extractInlines walks goldmark's inline children of an inline-bearing
// block (paragraph, heading, table cell, etc.) and returns a Lua table
// containing inline-node tables. The structure mirrors goldmark's AST:
// emphasis/strong/strikethrough/link become container inlines with their
// own `inlines` arrays; code spans and raw HTML become leaf inlines with a
// `text` field; soft and hard line breaks are emitted as synthetic inline
// nodes after the text whose flag is set; task checkboxes are skipped
// (their state is captured separately by detectTaskCheckbox).
func (r *Runtime) extractInlines(parent ast.Node, source []byte) *lua.LTable {
	out := r.L.NewTable()
	idx := 1
	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		idx = r.appendInlines(out, child, source, idx)
	}
	return out
}

// appendInlines emits Lua tables for the given goldmark inline node
// (and any synthetic break that follows) into out, returning the next
// index to use.
func (r *Runtime) appendInlines(out *lua.LTable, n ast.Node, source []byte, idx int) int {
	switch n := n.(type) {
	case *ast.Text:
		return r.appendTextInline(out, n, source, idx)
	case *ast.String:
		if s := string(n.Value); s != "" {
			out.RawSetInt(idx, r.makeLeafInline(inlineTypeText, s))
			return idx + 1
		}
		return idx
	case *ast.CodeSpan:
		var sb strings.Builder
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			collectRawText(&sb, c, source)
		}
		out.RawSetInt(idx, r.makeLeafInline(inlineTypeCodeSpan, sb.String()))
		// Note: the original fence width is not preserved on the AST.
		// renderInlineNode uses the smallest safe fence (computed from
		// the content) so a span whose content contains a backtick can
		// still round-trip without shifting in meaning.
		return idx + 1
	case *ast.RawHTML:
		var sb strings.Builder
		segs := n.Segments
		for i := range segs.Len() {
			seg := segs.At(i)
			sb.Write(seg.Value(source))
		}
		out.RawSetInt(idx, r.makeLeafInline(inlineTypeRawHTML, sb.String()))
		return idx + 1
	case *ast.AutoLink:
		an := r.L.NewTable()
		an.RawSetString("type", lua.LString(inlineTypeAutolink))
		an.RawSetString("url", lua.LString(string(n.URL(source))))
		out.RawSetInt(idx, an)
		return idx + 1
	case *ast.Link:
		out.RawSetInt(idx, r.makeLinkInline(n.Destination, n.Title, n, source))
		return idx + 1
	case *ast.Image:
		out.RawSetInt(idx, r.makeImageInline(n.Destination, n.Title, n, source))
		return idx + 1
	case *ast.Emphasis:
		kind := inlineTypeEmphasis
		if n.Level >= 2 {
			kind = inlineTypeStrong
		}
		out.RawSetInt(idx, r.makeContainerInline(kind, n, source))
		return idx + 1
	case *east.Strikethrough:
		out.RawSetInt(idx, r.makeContainerInline(inlineTypeStrikethrough, n, source))
		return idx + 1
	case *east.TaskCheckBox:
		// State is captured separately by detectTaskCheckbox.
		return idx
	default:
		// Unknown inline kind. Recurse to capture inner text.
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			idx = r.appendInlines(out, c, source, idx)
		}
		return idx
	}
}

// appendTextInline emits the text leaf and any soft/hard break following.
func (r *Runtime) appendTextInline(out *lua.LTable, n *ast.Text, source []byte, idx int) int {
	if seg := string(n.Segment.Value(source)); seg != "" {
		out.RawSetInt(idx, r.makeLeafInline(inlineTypeText, seg))
		idx++
	}
	switch {
	case n.HardLineBreak():
		b := r.L.NewTable()
		b.RawSetString("type", lua.LString(inlineTypeHardBreak))
		out.RawSetInt(idx, b)
		idx++
	case n.SoftLineBreak():
		b := r.L.NewTable()
		b.RawSetString("type", lua.LString(inlineTypeSoftBreak))
		out.RawSetInt(idx, b)
		idx++
	}
	return idx
}

// makeLeafInline constructs an inline node with `type` and `text` fields.
func (r *Runtime) makeLeafInline(kind inlineKind, text string) *lua.LTable {
	t := r.L.NewTable()
	t.RawSetString("type", lua.LString(kind))
	t.RawSetString("text", lua.LString(text))
	return t
}

// makeContainerInline constructs an emphasis/strong/strikethrough/link
// inline whose body is the inline tree of n's children.
func (r *Runtime) makeContainerInline(kind inlineKind, n ast.Node, source []byte) *lua.LTable {
	t := r.L.NewTable()
	t.RawSetString("type", lua.LString(kind))
	t.RawSetString("inlines", r.extractInlines(n, source))
	return t
}

// makeLinkInline constructs a link inline.
func (r *Runtime) makeLinkInline(dest, title []byte, n ast.Node, source []byte) *lua.LTable {
	t := r.L.NewTable()
	t.RawSetString("type", lua.LString(inlineTypeLink))
	t.RawSetString("url", lua.LString(string(dest)))
	if len(title) > 0 {
		t.RawSetString("title", lua.LString(string(title)))
	}
	t.RawSetString("inlines", r.extractInlines(n, source))
	return t
}

// makeImageInline constructs an image inline (alt content goes in
// `alt_inlines`).
func (r *Runtime) makeImageInline(dest, title []byte, n ast.Node, source []byte) *lua.LTable {
	t := r.L.NewTable()
	t.RawSetString("type", lua.LString(inlineTypeImage))
	t.RawSetString("url", lua.LString(string(dest)))
	if len(title) > 0 {
		t.RawSetString("title", lua.LString(string(title)))
	}
	t.RawSetString("alt_inlines", r.extractInlines(n, source))
	return t
}

// inlineKindOf reads the `type` field off an inline node table.
func inlineKindOf(node *lua.LTable) inlineKind {
	s, _ := node.RawGetString("type").(lua.LString)
	return inlineKind(s)
}

// blockKindOf reads the `type` field off a block node table.
func blockKindOf(node *lua.LTable) blockKind {
	s, _ := node.RawGetString("type").(lua.LString)
	return blockKind(s)
}

// collectRawText concatenates the raw text of leaf inline children. Used
// for code spans, where the inner content is meant to be opaque.
func collectRawText(sb *strings.Builder, n ast.Node, source []byte) {
	switch n := n.(type) {
	case *ast.Text:
		sb.Write(n.Segment.Value(source))
	case *ast.String:
		sb.Write(n.Value)
	default:
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			collectRawText(sb, c, source)
		}
	}
}

// renderInlines walks an inlines array and produces a markdown string
// preserving link/image/autolink/raw-HTML syntax. Used by block renderers
// so parse → render is a fixed point on the corpus.
func renderInlines(inlines *lua.LTable) string {
	if inlines == nil {
		return ""
	}
	var sb strings.Builder
	for i := 1; i <= inlines.Len(); i++ {
		v := inlines.RawGetInt(i)
		switch x := v.(type) {
		case lua.LString:
			// Tolerate a raw string inline (script convenience).
			sb.WriteString(string(x))
		case *lua.LTable:
			renderInlineNode(&sb, x)
		}
	}
	return sb.String()
}

func renderInlineNode(sb *strings.Builder, node *lua.LTable) {
	switch inlineKindOf(node) {
	case inlineTypeText, inlineTypeRawHTML:
		writeStringField(sb, node, "text")
	case inlineTypeCodeSpan:
		s, _ := node.RawGetString("text").(lua.LString)
		writeCodeSpan(sb, string(s))
	case inlineTypeAutolink:
		sb.WriteByte('<')
		writeStringField(sb, node, "url")
		sb.WriteByte('>')
	case inlineTypeSoftBreak:
		sb.WriteByte('\n')
	case inlineTypeHardBreak:
		sb.WriteString("  \n")
	case inlineTypeEmphasis:
		sb.WriteByte('*')
		writeInlinesOrFallback(sb, node, renderInlines)
		sb.WriteByte('*')
	case inlineTypeStrong:
		sb.WriteString("**")
		writeInlinesOrFallback(sb, node, renderInlines)
		sb.WriteString("**")
	case inlineTypeStrikethrough:
		sb.WriteString("~~")
		writeInlinesOrFallback(sb, node, renderInlines)
		sb.WriteString("~~")
	case inlineTypeLink:
		renderLinkInline(sb, node)
	case inlineTypeImage:
		renderImageInline(sb, node)
	default:
		// Unknown inline: emit `text` if present (defensive).
		writeStringField(sb, node, "text")
	}
}

// writeStringField writes node[field] as a string if it's a Lua string.
func writeStringField(sb *strings.Builder, node *lua.LTable, field string) {
	if s, ok := node.RawGetString(field).(lua.LString); ok {
		sb.WriteString(string(s))
	}
}

// inlinesField reads node[field] as an inlines/children table, returning
// nil if the field is missing or not a table.
func inlinesField(node *lua.LTable, field string) *lua.LTable {
	t, _ := node.RawGetString(field).(*lua.LTable)
	return t
}

// stringField reads node[field] as a Go string, returning "" if the
// field is missing or not a string.
func stringField(node *lua.LTable, field string) string {
	s, _ := node.RawGetString(field).(lua.LString)
	return string(s)
}

// writeCodeSpan emits a code span using the smallest backtick fence that
// is longer than any run of backticks inside the content (CommonMark
// 6.1). If the content starts or ends with a backtick, a single space is
// added inside the fence on each side; the parser strips this space at
// re-parse, so the round-trip is preserved.
func writeCodeSpan(sb *strings.Builder, content string) {
	// Find the longest run of backticks in content.
	maxRun, run := 0, 0
	for i := range len(content) {
		if content[i] == '`' {
			run++
			if run > maxRun {
				maxRun = run
			}
		} else {
			run = 0
		}
	}
	fenceLen := maxRun + 1
	fence := strings.Repeat("`", fenceLen)
	sb.WriteString(fence)
	pad := content != "" && (content[0] == '`' || content[len(content)-1] == '`')
	if pad {
		sb.WriteByte(' ')
	}
	sb.WriteString(content)
	if pad {
		sb.WriteByte(' ')
	}
	sb.WriteString(fence)
}

// renderLinkInline emits "[text](url)" or "[text](url \"title\")".
func renderLinkInline(sb *strings.Builder, node *lua.LTable) {
	url, _ := node.RawGetString("url").(lua.LString)
	title, _ := node.RawGetString("title").(lua.LString)
	sb.WriteByte('[')
	writeInlinesOrFallback(sb, node, renderInlines)
	sb.WriteString("](")
	writeLinkURLAndTitle(sb, string(url), string(title))
	sb.WriteByte(')')
}

// renderImageInline emits "![alt](url)" or "![alt](url \"title\")".
// Alt content uses flatten policy (no link/emphasis wrappers).
func renderImageInline(sb *strings.Builder, node *lua.LTable) {
	url, _ := node.RawGetString("url").(lua.LString)
	title, _ := node.RawGetString("title").(lua.LString)
	alt, _ := node.RawGetString("alt_inlines").(*lua.LTable)
	sb.WriteString("![")
	sb.WriteString(flattenInlines(alt))
	sb.WriteString("](")
	writeLinkURLAndTitle(sb, string(url), string(title))
	sb.WriteByte(')')
}

// writeLinkURLAndTitle emits the destination + optional title portion of
// a link or image. URLs that contain whitespace or unbalanced
// parentheses are wrapped in angle brackets per CommonMark.
func writeLinkURLAndTitle(sb *strings.Builder, url, title string) {
	if needsAngleBrackets(url) {
		sb.WriteByte('<')
		sb.WriteString(url)
		sb.WriteByte('>')
	} else {
		sb.WriteString(url)
	}
	if title != "" {
		sb.WriteString(` "`)
		sb.WriteString(title)
		sb.WriteByte('"')
	}
}

// needsAngleBrackets reports whether a URL must be wrapped in <...> to
// be valid in a markdown link destination. Whitespace, control chars,
// and unbalanced parens force the wrapper.
func needsAngleBrackets(url string) bool {
	depth := 0
	for i := range len(url) {
		c := url[i]
		switch {
		case c <= ' ':
			return true
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth < 0 {
				return true
			}
		}
	}
	return depth != 0
}

// flattenInlines applies the legacy text-extraction policy: strikethrough
// and code spans are preserved as `~~...~~` / “ `...` “; emphasis,
// strong, and link wrappers are dropped (only inner text); raw HTML is
// emitted verbatim; autolinks render as the URL; soft/hard breaks render
// as a single space (matching the pre-refactor behavior where soft
// breaks became spaces and inline structure was condensed to text).
//
// Used by helpers that previously read `node.text`: `headers`,
// `first_paragraph`, and the public `rela.md.flatten`.
func flattenInlines(inlines *lua.LTable) string {
	if inlines == nil {
		return ""
	}
	var sb strings.Builder
	for i := 1; i <= inlines.Len(); i++ {
		v := inlines.RawGetInt(i)
		switch x := v.(type) {
		case lua.LString:
			sb.WriteString(string(x))
		case *lua.LTable:
			flattenInlineNode(&sb, x)
		}
	}
	return sb.String()
}

func flattenInlineNode(sb *strings.Builder, node *lua.LTable) {
	switch inlineKindOf(node) {
	case inlineTypeText, inlineTypeRawHTML:
		if s, ok := node.RawGetString("text").(lua.LString); ok {
			sb.WriteString(string(s))
		}
	case inlineTypeCodeSpan:
		s, _ := node.RawGetString("text").(lua.LString)
		writeCodeSpan(sb, string(s))
	case inlineTypeAutolink:
		if s, ok := node.RawGetString("url").(lua.LString); ok {
			sb.WriteString(string(s))
		}
	case inlineTypeSoftBreak, inlineTypeHardBreak:
		sb.WriteByte(' ')
	case inlineTypeStrikethrough:
		sb.WriteString("~~")
		writeInlinesOrFallback(sb, node, flattenInlines)
		sb.WriteString("~~")
	case inlineTypeEmphasis, inlineTypeStrong, inlineTypeLink:
		// Drop wrapper, keep inner text.
		writeInlinesOrFallback(sb, node, flattenInlines)
	case inlineTypeImage:
		// Image: emit alt text (no URL).
		alt, _ := node.RawGetString("alt_inlines").(*lua.LTable)
		sb.WriteString(flattenInlines(alt))
	default:
		if s, ok := node.RawGetString("text").(lua.LString); ok {
			sb.WriteString(string(s))
		}
	}
}

// writeInlinesOrFallback writes the rendering of node.inlines via the
// supplied renderer (renderInlines or flattenInlines). If the node has
// no `inlines` field but has a legacy `text` value, that value is coerced
// to a string and written verbatim (compat with hand-built script
// tables, including numeric `text` values).
func writeInlinesOrFallback(sb *strings.Builder, node *lua.LTable, render func(*lua.LTable) string) {
	if t := inlinesField(node, "inlines"); t != nil {
		sb.WriteString(render(t))
		return
	}
	if v := node.RawGetString("text"); v != lua.LNil {
		sb.WriteString(lua.LVAsString(v))
	}
}

// extractCodeBlockContent extracts content from a fenced code block.
func (r *Runtime) extractCodeBlockContent(fcb *ast.FencedCodeBlock, source []byte) string {
	var sb strings.Builder
	lines := fcb.Lines()
	for i := range lines.Len() {
		line := lines.At(i)
		sb.Write(line.Value(source))
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

// extractLinesContent extracts content from a node with Lines().
func (r *Runtime) extractLinesContent(n ast.Node, source []byte) string {
	var sb strings.Builder
	if ln, ok := n.(interface{ Lines() *text.Segments }); ok {
		lines := ln.Lines()
		for i := range lines.Len() {
			line := lines.At(i)
			sb.Write(line.Value(source))
		}
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

// extractListItems extracts list items as a Lua table. Each item is one
// of:
//
//   - lua.LString: a plain item whose body is a single paragraph
//     containing only text (the simple case — matches the historical
//     shape so simple bullet/numbered lists are ergonomic).
//   - *lua.LTable with `inlines`: a plain item whose body is a single
//     paragraph with formatting (links, code spans, etc.).
//   - *lua.LTable with `children`: a multi-block item (e.g. an item
//     containing a fenced code block, a nested list, or multiple
//     paragraphs).
//   - Task items: a *lua.LTable with `task=true`, `checked=bool`,
//     plus `inlines` or `children` per the same rules.
//
// Task-checkbox state is captured by detectTaskCheckbox; the inline
// extractor skips the TaskCheckBox node so `inlines`/`children` do not
// contain a phantom checkbox.
func (r *Runtime) extractListItems(n ast.Node, source []byte) *lua.LTable {
	items := r.L.NewTable()
	idx := 1
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if child.Kind() != ast.KindListItem {
			continue
		}
		isTask, checked := detectTaskCheckbox(child)

		// Count and classify the item's block children.
		var blocks []ast.Node
		for c := child.FirstChild(); c != nil; c = c.NextSibling() {
			blocks = append(blocks, c)
		}

		switch {
		case isTask:
			item := r.L.NewTable()
			item.RawSetString("task", lua.LBool(true))
			item.RawSetString("checked", lua.LBool(checked))
			r.attachItemBody(item, blocks, source)
			items.RawSetInt(idx, item)
		case len(blocks) == 1 && isParagraphLike(blocks[0]) && isSimpleTextOnly(blocks[0]):
			// Plain item with simple text only — return as LString.
			items.RawSetInt(idx, lua.LString(simpleParagraphText(blocks[0], source)))
		case len(blocks) == 1 && isParagraphLike(blocks[0]):
			// Single-paragraph plain item with formatting.
			item := r.L.NewTable()
			item.RawSetString("inlines", r.extractInlines(blocks[0], source))
			items.RawSetInt(idx, item)
		default:
			// Multi-block item.
			item := r.L.NewTable()
			children := r.L.NewTable()
			for i, b := range blocks {
				if bn := r.nodeToLua(b, source); bn != nil {
					children.RawSetInt(i+1, bn)
				}
			}
			item.RawSetString("children", children)
			items.RawSetInt(idx, item)
		}
		idx++
	}
	return items
}

// attachItemBody chooses between `inlines` (single-paragraph item) and
// `children` (multi-block item) for a list-item table.
func (r *Runtime) attachItemBody(item *lua.LTable, blocks []ast.Node, source []byte) {
	if len(blocks) == 1 && isParagraphLike(blocks[0]) {
		item.RawSetString("inlines", r.extractInlines(blocks[0], source))
		return
	}
	children := r.L.NewTable()
	idx := 1
	for _, b := range blocks {
		if bn := r.nodeToLua(b, source); bn != nil {
			children.RawSetInt(idx, bn)
			idx++
		}
	}
	item.RawSetString("children", children)
}

// isParagraphLike reports whether a goldmark block node is a paragraph
// or text block (the two block kinds that carry inline content directly).
func isParagraphLike(n ast.Node) bool {
	k := n.Kind()
	return k == ast.KindParagraph || k == ast.KindTextBlock
}

// simpleParagraphText extracts the literal concatenated text of a
// paragraph/text-block whose inline tree is all Text/String nodes (the
// fast path for plain list items). Caller should already have verified
// this via isSimpleTextOnly.
func simpleParagraphText(n ast.Node, source []byte) string {
	var sb strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c := c.(type) {
		case *ast.Text:
			sb.Write(c.Segment.Value(source))
		case *ast.String:
			sb.Write(c.Value)
		}
	}
	return strings.TrimLeft(sb.String(), " \t")
}

// isSimpleTextOnly reports whether a paragraph/text-block's inline tree
// contains only Text nodes (no formatting, links, code spans, etc.). When
// true the caller can compress the item to a single string.
func isSimpleTextOnly(n ast.Node) bool {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c := c.(type) {
		case *ast.Text:
			if c.HardLineBreak() || c.SoftLineBreak() {
				return false
			}
		case *ast.String:
			// fine
		case *east.TaskCheckBox:
			// skipped by extractor; doesn't disqualify
		default:
			return false
		}
	}
	return true
}

// extractBlockChildren walks the immediate block-level children of a
// container (blockquote, list-item) and returns them as an array of
// block-node Lua tables.
func (r *Runtime) extractBlockChildren(parent ast.Node, source []byte) *lua.LTable {
	out := r.L.NewTable()
	idx := 1
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		if bn := r.nodeToLua(c, source); bn != nil {
			out.RawSetInt(idx, bn)
			idx++
		}
	}
	return out
}

// detectTaskCheckbox returns whether the list item is a task item and its
// checked state. Per the GFM task list spec, the TaskCheckBox must be the
// first inline node of the FIRST TextBlock/Paragraph child of the ListItem.
// We do not scan subsequent siblings — a checkbox in a later block would
// not be parsed as a task marker by goldmark anyway.
func detectTaskCheckbox(li ast.Node) (isTask, checked bool) {
	for c := li.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Kind() != ast.KindTextBlock && c.Kind() != ast.KindParagraph {
			continue
		}
		for inline := c.FirstChild(); inline != nil; inline = inline.NextSibling() {
			if cb, ok := inline.(*east.TaskCheckBox); ok {
				return true, cb.IsChecked
			}
		}
		return false, false
	}
	return false, false
}

// extractRawContent extracts raw source content for unsupported nodes.
func (r *Runtime) extractRawContent(n ast.Node, source []byte) string {
	// Try to get the lines if available
	if ln, ok := n.(interface{ Lines() *text.Segments }); ok {
		lines := ln.Lines()
		if lines.Len() > 0 {
			start := lines.At(0).Start
			end := lines.At(lines.Len() - 1).Stop
			return string(source[start:end])
		}
	}
	return ""
}

// extractTableData extracts header, rows, and alignments from a GFM table
// node. Each cell is an `inlines` array (preserving link/code/raw HTML
// structure inside the cell).
func (r *Runtime) extractTableData(table *east.Table, source []byte) (header, rows, alignments *lua.LTable) {
	header = r.L.NewTable()
	rows = r.L.NewTable()
	alignments = r.L.NewTable()

	// Extract alignments from table columns
	for i, a := range table.Alignments {
		var align string
		switch a {
		case east.AlignLeft:
			align = alignLeft
		case east.AlignRight:
			align = alignRight
		case east.AlignCenter:
			align = alignCenter
		default:
			align = alignNone
		}
		alignments.RawSetInt(i+1, lua.LString(align))
	}

	// Walk children: TableHeader contains the header row, TableRows are data rows.
	// Each cell becomes an inlines array (preserves links/code/raw HTML inside).
	rowIdx := 1
	for child := table.FirstChild(); child != nil; child = child.NextSibling() {
		switch child.Kind() {
		case east.KindTableHeader:
			cellIdx := 1
			for cell := child.FirstChild(); cell != nil; cell = cell.NextSibling() {
				header.RawSetInt(cellIdx, r.extractInlines(cell, source))
				cellIdx++
			}
		case east.KindTableRow:
			luaRow := r.L.NewTable()
			cellIdx := 1
			for cell := child.FirstChild(); cell != nil; cell = cell.NextSibling() {
				luaRow.RawSetInt(cellIdx, r.extractInlines(cell, source))
				cellIdx++
			}
			rows.RawSetInt(rowIdx, luaRow)
			rowIdx++
		}
	}

	return header, rows, alignments
}

// shiftNodeHeaders creates a deep copy of a node with shifted header levels.
func (r *Runtime) shiftNodeHeaders(node *lua.LTable, offset int) *lua.LTable {
	newNode := r.deepCopyNode(node)

	if nodeType := newNode.RawGetString("type"); nodeType == lua.LString(nodeTypeHeading) {
		if level, ok := newNode.RawGetString("level").(lua.LNumber); ok {
			newLevel := int(level) + offset
			if newLevel < minHeaderLevel {
				newLevel = minHeaderLevel
			}
			if newLevel > maxHeaderLevel {
				newLevel = maxHeaderLevel
			}
			newNode.RawSetString("level", lua.LNumber(newLevel))
		}
	}

	return newNode
}

// deepCopyNode creates a deep copy of a node table, including nested tables.
func (r *Runtime) deepCopyNode(node *lua.LTable) *lua.LTable {
	newNode := r.L.NewTable()
	node.ForEach(func(k, v lua.LValue) {
		if tbl, ok := v.(*lua.LTable); ok {
			newNode.RawSet(k, r.deepCopyTable(tbl))
		} else {
			newNode.RawSet(k, v)
		}
	})
	return newNode
}

// deepCopyTable creates a deep copy of a Lua table.
func (r *Runtime) deepCopyTable(tbl *lua.LTable) *lua.LTable {
	newTbl := r.L.NewTable()
	tbl.ForEach(func(k, v lua.LValue) {
		if nested, ok := v.(*lua.LTable); ok {
			newTbl.RawSet(k, r.deepCopyTable(nested))
		} else {
			newTbl.RawSet(k, v)
		}
	})
	return newTbl
}

// renderNodes renders AST nodes to markdown.
func renderNodes(sb *strings.Builder, nodes *lua.LTable) {
	// Use sequential access to preserve document order
	for i := 1; i <= nodes.Len(); i++ {
		v := nodes.RawGetInt(i)
		node, ok := v.(*lua.LTable)
		if !ok {
			continue
		}
		if i > 1 {
			sb.WriteString("\n")
		}
		renderNode(sb, node)
	}
}

// renderNode renders a single AST node to markdown.
func renderNode(sb *strings.Builder, node *lua.LTable) {
	switch blockKindOf(node) {
	case nodeTypeHeading:
		renderHeading(sb, node)
	case nodeTypeParagraph:
		renderParagraph(sb, node)
	case nodeTypeCodeBlock:
		renderCodeBlock(sb, node)
	case nodeTypeList:
		renderList(sb, node)
	case nodeTypeBlockquote:
		renderBlockquote(sb, node)
	case nodeTypeThematicBreak:
		sb.WriteString("---\n")
	case nodeTypeTable:
		renderTableNode(sb, node)
	case nodeTypeRaw:
		renderRaw(sb, node)
	}
}

// renderHeading renders a heading node. Headings can't contain hard or
// soft breaks in CommonMark; if the inlines table contains break nodes
// (e.g., a script copied them in from a paragraph), they are flattened
// to spaces so the heading stays on one line.
func renderHeading(sb *strings.Builder, node *lua.LTable) {
	level := 1
	if l, ok := node.RawGetString("level").(lua.LNumber); ok {
		level = int(l)
	}
	sb.WriteString(strings.Repeat("#", level))
	sb.WriteByte(' ')
	if t := inlinesField(node, "inlines"); t != nil {
		// Render breaks as spaces; otherwise full syntax preservation.
		s := renderInlines(t)
		s = strings.ReplaceAll(s, "  \n", " ")
		s = strings.ReplaceAll(s, "\n", " ")
		sb.WriteString(s)
	} else if v := node.RawGetString("text"); v != lua.LNil {
		sb.WriteString(lua.LVAsString(v))
	}
	sb.WriteByte('\n')
}

// renderParagraph renders a paragraph node.
func renderParagraph(sb *strings.Builder, node *lua.LTable) {
	writeInlinesOrFallback(sb, node, renderInlines)
	sb.WriteByte('\n')
}

// renderCodeBlock renders a code block node.
func renderCodeBlock(sb *strings.Builder, node *lua.LTable) {
	language := ""
	if l, ok := node.RawGetString("language").(lua.LString); ok {
		language = string(l)
	}
	content := ""
	if c, ok := node.RawGetString("content").(lua.LString); ok {
		content = string(c)
	}
	sb.WriteString("```")
	sb.WriteString(language)
	sb.WriteString("\n")
	sb.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("```\n")
}

// renderList renders a list node. Items may be plain strings or tables.
// Tables with task=true render with [x] / [ ] checkboxes. Multi-block
// items (with `children`) get continuation lines indented to align under
// the marker.
//
// Items must be stored at sequential 1..N indices. Sparse tables (e.g.,
// items[2] = nil to "delete" an item) will truncate at the first gap
// because Lua's table length operator returns a "border", not a count.
// Scripts that mutate items in place should compact the table afterwards.
func renderList(sb *strings.Builder, node *lua.LTable) {
	ordered := false
	if o, ok := node.RawGetString("ordered").(lua.LBool); ok {
		ordered = bool(o)
	}
	items, ok := node.RawGetString("items").(*lua.LTable)
	if !ok {
		return
	}
	for i := 1; i <= items.Len(); i++ {
		v := items.RawGetInt(i)
		if v == lua.LNil {
			continue
		}
		marker := "- "
		if ordered {
			marker = fmt.Sprintf("%d. ", i)
		}
		renderListItem(sb, v, marker)
	}
}

// renderListItem writes one list item using the given marker (e.g. "- "
// or "1. "). It chooses the right rendering path based on the item's
// shape (LString | table-with-inlines | table-with-children | task).
func renderListItem(sb *strings.Builder, v lua.LValue, marker string) {
	switch item := v.(type) {
	case lua.LString:
		sb.WriteString(marker)
		sb.WriteString(string(item))
		sb.WriteByte('\n')
		return
	case *lua.LTable:
		taskPrefix := ""
		if isTaskItem(item) {
			checked := false
			if c, ok := item.RawGetString("checked").(lua.LBool); ok {
				checked = bool(c)
			}
			if checked {
				taskPrefix = "[x] "
			} else {
				taskPrefix = "[ ] "
			}
		}

		// Multi-block: render each child with continuation indent. Empty
		// continuation lines get no indent — emitting whitespace-only
		// lines would (a) trip "no trailing whitespace" linters and (b)
		// risk the two-trailing-spaces hard-break interpretation.
		if children := inlinesField(item, "children"); children != nil && children.Len() > 0 {
			indent := strings.Repeat(" ", len(marker))
			var inner strings.Builder
			renderNodes(&inner, children)
			rendered := inner.String()
			lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
			for li, line := range lines {
				switch {
				case li == 0:
					sb.WriteString(marker)
					sb.WriteString(taskPrefix)
					sb.WriteString(line)
				case line == "":
					// Empty continuation line: emit no indent.
				default:
					sb.WriteString(indent)
					sb.WriteString(line)
				}
				sb.WriteByte('\n')
			}
			return
		}

		// Single-paragraph item with `inlines` (or legacy `text`).
		sb.WriteString(marker)
		sb.WriteString(taskPrefix)
		writeInlinesOrFallback(sb, item, renderInlines)
		sb.WriteByte('\n')
		return
	}
	// Unknown item type — emit an empty bullet rather than crashing.
	sb.WriteString(marker)
	sb.WriteByte('\n')
}

// isTaskItem reports whether a list item table represents a task item.
// Only an explicit lua.LBool(true) qualifies.
func isTaskItem(item *lua.LTable) bool {
	v, ok := item.RawGetString("task").(lua.LBool)
	return ok && bool(v)
}

// renderBlockquote renders a blockquote node by recursively rendering its
// block children and prefixing every output line with "> ".
func renderBlockquote(sb *strings.Builder, node *lua.LTable) {
	children := inlinesField(node, "children")
	if children == nil {
		// Compatibility: hand-built blockquote with `inlines` (single
		// paragraph) or legacy `content` string.
		var inner strings.Builder
		writeInlinesOrFallback(&inner, node, renderInlines)
		if c, ok := node.RawGetString("content").(lua.LString); ok && inner.Len() == 0 {
			inner.WriteString(string(c))
		}
		prefixLines(sb, "> ", inner.String())
		return
	}
	var inner strings.Builder
	renderNodes(&inner, children)
	prefixLines(sb, "> ", inner.String())
}

// prefixLines writes lines from s to sb, prefixing each line with
// prefix. A trailing newline in s does not produce an empty prefixed
// line. Empty inner lines get the prefix with trailing whitespace
// trimmed (so blockquote continuations don't emit "> " on blank lines,
// and list-indent doesn't emit lines containing only spaces).
func prefixLines(sb *strings.Builder, prefix, s string) {
	if s == "" {
		sb.WriteString(strings.TrimRight(prefix, " \t"))
		sb.WriteByte('\n')
		return
	}
	end := len(s)
	if strings.HasSuffix(s, "\n") {
		end--
	}
	body := s[:end]
	trimmed := strings.TrimRight(prefix, " \t")
	for _, line := range strings.Split(body, "\n") {
		if line == "" {
			sb.WriteString(trimmed)
		} else {
			sb.WriteString(prefix)
			sb.WriteString(line)
		}
		sb.WriteByte('\n')
	}
}

// renderRaw renders a raw node.
func renderRaw(sb *strings.Builder, node *lua.LTable) {
	content := ""
	if c, ok := node.RawGetString("content").(lua.LString); ok {
		content = string(c)
	}
	sb.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		sb.WriteString("\n")
	}
}

// renderTableNode renders a table AST node to GFM markdown with
// column-aligned padding. Cells are inline arrays and are flattened to
// rendered markdown via renderInlines (preserves links/code/raw HTML).
func renderTableNode(sb *strings.Builder, node *lua.LTable) {
	header, _ := node.RawGetString("header").(*lua.LTable)
	rows, _ := node.RawGetString("rows").(*lua.LTable)
	alignments, _ := node.RawGetString("alignments").(*lua.LTable)

	if header == nil || header.Len() == 0 {
		return
	}

	numCols := header.Len()
	headerCells := make([]string, numCols)
	colWidths := make([]int, numCols)
	colAligns := make([]string, numCols)

	for i := range numCols {
		headerCells[i] = renderTableCell(header.RawGetInt(i + 1))
		colWidths[i] = runewidth.StringWidth(headerCells[i])
		colAligns[i] = alignNone
		if alignments != nil {
			if a, ok := alignments.RawGetInt(i + 1).(lua.LString); ok {
				colAligns[i] = string(a)
			}
		}
	}

	var dataRows [][]string
	if rows != nil {
		for i := 1; i <= rows.Len(); i++ {
			row, ok := rows.RawGetInt(i).(*lua.LTable)
			if !ok {
				continue
			}
			cells := make([]string, numCols)
			for j := range numCols {
				cells[j] = renderTableCell(row.RawGetInt(j + 1))
				if runewidth.StringWidth(cells[j]) > colWidths[j] {
					colWidths[j] = runewidth.StringWidth(cells[j])
				}
			}
			dataRows = append(dataRows, cells)
		}
	}

	// Ensure minimum width of 3 for separator dashes
	for i := range colWidths {
		if colWidths[i] < 3 {
			colWidths[i] = 3
		}
	}

	// Header row
	sb.WriteString("|")
	for i := range numCols {
		sb.WriteString(" ")
		sb.WriteString(padCell(headerCells[i], colWidths[i], colAligns[i]))
		sb.WriteString(" |")
	}
	sb.WriteString("\n")

	// Separator row
	sb.WriteString("|")
	for i := range numCols {
		sb.WriteString(" ")
		sb.WriteString(renderSeparator(colWidths[i], colAligns[i]))
		sb.WriteString(" |")
	}
	sb.WriteString("\n")

	// Data rows
	for _, cells := range dataRows {
		sb.WriteString("|")
		for j := range numCols {
			sb.WriteString(" ")
			sb.WriteString(padCell(cells[j], colWidths[j], colAligns[j]))
			sb.WriteString(" |")
		}
		sb.WriteString("\n")
	}
}

// renderTableCell coerces a cell value to its rendered markdown form.
// Cells are normally inline arrays (post-refactor); accept LStrings as a
// compatibility shim for hand-built tables. Pipes are escaped and
// newlines collapsed because GFM tables are line-based: an unescaped
// pipe splits the row, and a newline terminates it.
func renderTableCell(v lua.LValue) string {
	var s string
	switch x := v.(type) {
	case *lua.LTable:
		s = renderInlines(x)
	case lua.LString:
		s = string(x)
	default:
		s = lua.LVAsString(v)
	}
	return escapeTableCell(s)
}

// escapeTableCell escapes characters that would corrupt a GFM table
// row: `|` → `\|`, and any embedded newline → space (GFM does not allow
// multi-line cells; the alternative is `<br>` but that requires the
// renderer to know whether HTML is acceptable).
func escapeTableCell(s string) string {
	if !strings.ContainsAny(s, "|\n\r") {
		return s
	}
	const slack = 4 // small headroom for escaped pipes
	var b strings.Builder
	b.Grow(len(s) + slack)
	for _, r := range s {
		switch r {
		case '|':
			b.WriteString(`\|`)
		case '\n', '\r':
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// padCell pads a cell value to the given width based on alignment.
// Width is measured in display columns (using runewidth) to handle
// multi-byte characters like CJK and emoji correctly.
func padCell(value string, width int, align string) string {
	padding := width - runewidth.StringWidth(value)
	if padding <= 0 {
		return value
	}
	switch align {
	case alignRight:
		return strings.Repeat(" ", padding) + value
	case alignCenter:
		left := padding / 2
		right := padding - left
		return strings.Repeat(" ", left) + value + strings.Repeat(" ", right)
	default:
		return value + strings.Repeat(" ", padding)
	}
}

// renderSeparator renders a table separator cell with alignment markers.
// width must be >= 3 (enforced by renderTableNode's minimum column width).
func renderSeparator(width int, align string) string {
	if width < 3 {
		width = 3
	}
	switch align {
	case alignLeft:
		return ":" + strings.Repeat("-", width-1)
	case alignRight:
		return strings.Repeat("-", width-1) + ":"
	case alignCenter:
		return ":" + strings.Repeat("-", width-2) + ":"
	default:
		return strings.Repeat("-", width)
	}
}

// --- Markdown Generation Helpers ---

// luaMdLink implements rela.md.link(text, url) -> string
// Returns a markdown link: [text](url)
func luaMdLink(ls *lua.LState) int {
	linkText := ls.CheckString(1)
	url := ls.CheckString(2)
	ls.Push(lua.LString("[" + linkText + "](" + url + ")"))
	return 1
}

// luaMdRef implements rela.md.ref(id, text) -> string
// Returns a markdown reference link: [text][id]
func luaMdRef(ls *lua.LState) int {
	id := ls.CheckString(1)
	refText := ls.CheckString(2)
	ls.Push(lua.LString("[" + refText + "][" + id + "]"))
	return 1
}

// luaMdTable implements rela.md.table(headers, rows) -> string
// Builds a markdown table from headers and rows.
func luaMdTable(ls *lua.LState) int {
	headers := ls.CheckTable(1)
	rows := ls.CheckTable(2)

	var sb strings.Builder

	// Build header row
	sb.WriteString("|")
	for i := 1; i <= headers.Len(); i++ {
		h := headers.RawGetInt(i)
		sb.WriteString(" ")
		sb.WriteString(lua.LVAsString(h))
		sb.WriteString(" |")
	}
	sb.WriteString("\n")

	// Build separator row
	sb.WriteString("|")
	for i := 1; i <= headers.Len(); i++ {
		sb.WriteString(" -------- |")
	}
	sb.WriteString("\n")

	// Build data rows
	for i := 1; i <= rows.Len(); i++ {
		rowVal := rows.RawGetInt(i)
		row, ok := rowVal.(*lua.LTable)
		if !ok {
			continue
		}
		sb.WriteString("|")
		for j := 1; j <= row.Len(); j++ {
			cell := row.RawGetInt(j)
			sb.WriteString(" ")
			sb.WriteString(lua.LVAsString(cell))
			sb.WriteString(" |")
		}
		sb.WriteString("\n")
	}

	ls.Push(lua.LString(sb.String()))
	return 1
}

// luaMdEntityTable implements rela.md.entity_table(entities, columns) -> string
// Builds a markdown table from entities using column specifications.
// Column spec: {"Header", "field"} or {"Header", "field", "default"} or {"Header", function}
func (r *Runtime) luaMdEntityTable(ls *lua.LState) int {
	entities := ls.CheckTable(1)
	columns := ls.CheckTable(2)

	var sb strings.Builder

	// Build header row
	sb.WriteString("|")
	for i := 1; i <= columns.Len(); i++ {
		colVal := columns.RawGetInt(i)
		col, ok := colVal.(*lua.LTable)
		if !ok {
			continue
		}
		header := lua.LVAsString(col.RawGetInt(1))
		sb.WriteString(" ")
		sb.WriteString(header)
		sb.WriteString(" |")
	}
	sb.WriteString("\n")

	// Build separator row
	sb.WriteString("|")
	for i := 1; i <= columns.Len(); i++ {
		sb.WriteString(" -------- |")
	}
	sb.WriteString("\n")

	// Build data rows
	for i := 1; i <= entities.Len(); i++ {
		entityVal := entities.RawGetInt(i)
		entity, ok := entityVal.(*lua.LTable)
		if !ok {
			continue
		}

		sb.WriteString("|")
		for j := 1; j <= columns.Len(); j++ {
			colVal := columns.RawGetInt(j)
			col, ok := colVal.(*lua.LTable)
			if !ok {
				sb.WriteString(" |")
				continue
			}

			cellValue := r.evalColumnSpec(ls, col, entity)
			sb.WriteString(" ")
			sb.WriteString(cellValue)
			sb.WriteString(" |")
		}
		sb.WriteString("\n")
	}

	ls.Push(lua.LString(sb.String()))
	return 1
}

// evalColumnSpec evaluates a column specification against an entity.
// Spec: {"Header", "field"} or {"Header", "field", "default"} or {"Header", function}
func (r *Runtime) evalColumnSpec(ls *lua.LState, col, entity *lua.LTable) string {
	spec := col.RawGetInt(2)

	switch s := spec.(type) {
	case *lua.LFunction:
		// Call the function with entity as argument
		ls.Push(s)
		ls.Push(entity)
		if err := ls.PCall(1, 1, nil); err != nil {
			return ""
		}
		result := ls.Get(-1)
		ls.Pop(1)
		return lua.LVAsString(result)

	case lua.LString:
		// Property name - look up in entity.properties
		fieldName := string(s)
		defaultVal := ""
		if col.Len() >= 3 {
			defaultVal = lua.LVAsString(col.RawGetInt(3))
		}

		// Get properties table
		propsVal := entity.RawGetString("properties")
		props, ok := propsVal.(*lua.LTable)
		if !ok {
			return defaultVal
		}

		// Get property value
		val := props.RawGetString(fieldName)
		if val == lua.LNil {
			return defaultVal
		}
		valStr := lua.LVAsString(val)
		if valStr == "" {
			return defaultVal
		}
		return valStr

	default:
		return ""
	}
}

// --- Reference Resolution ---
//
// resolve_refs and entity_refs work together to turn entity-ID code spans
// in markdown content into links. The match rule is deliberately narrow:
// only `` `<id>` `` literals (i.e. inline code spans whose entire content
// is an entity ID) are rewritten. Bare prose mentions like `see TKT-1`
// are left alone because the boundary semantics are ambiguous and
// false-positive prone. Authors opt in by writing the ID as a code span,
// which is the existing convention for "this is an identifier, not
// prose."

// luaMdResolveRefs implements rela.md.resolve_refs(ast, replacements) -> ast.
//
// Walks the AST and replaces every `code_span` inline whose text matches
// a key in the `replacements` map with the rendered splice value.
// Splices are parsed as inline markdown so the replacement integrates
// structurally (a `[Title](url)` splice becomes a `link` inline, not a
// raw text fragment).
func (r *Runtime) luaMdResolveRefs(ls *lua.LState) int {
	astTable, ok := ls.Get(1).(*lua.LTable)
	if !ok {
		ls.RaiseError("rela.md.resolve_refs: ast must be a table")
		return 0
	}
	mapTable, ok := ls.Get(2).(*lua.LTable)
	if !ok {
		ls.RaiseError("rela.md.resolve_refs: replacements must be a table")
		return 0
	}

	replacements, err := r.collectRefReplacements(mapTable)
	if err != nil {
		ls.RaiseError("%s", err.Error())
		return 0
	}

	out := r.deepCopyAST(astTable)
	if len(replacements) > 0 {
		r.rewriteCodeSpanRefs(out, replacements)
	}
	ls.Push(out)
	return 1
}

// collectRefReplacements validates the replacements map. Keys must be
// non-empty strings, values must be strings without `\n`/`\r`. Pre-parses
// each value's inline tree so we don't re-parse N times during the walk.
func (r *Runtime) collectRefReplacements(mapTable *lua.LTable) (map[string]*lua.LTable, error) {
	replacements := make(map[string]*lua.LTable)
	var validationErr error
	mapTable.ForEach(func(k, v lua.LValue) {
		if validationErr != nil {
			return
		}
		ks, ok := k.(lua.LString)
		if !ok {
			validationErr = errors.New("rela.md.resolve_refs: replacement keys must be strings")
			return
		}
		key := string(ks)
		if key == "" {
			validationErr = errors.New("rela.md.resolve_refs: replacement keys must be non-empty")
			return
		}
		vs, ok := v.(lua.LString)
		if !ok {
			validationErr = fmt.Errorf(
				"rela.md.resolve_refs: replacement for %q must be a string", key)
			return
		}
		val := string(vs)
		if strings.ContainsAny(val, "\n\r") {
			validationErr = fmt.Errorf(
				"rela.md.resolve_refs: replacement for %q contains a newline; "+
					"use only inline markdown in replacement values", key)
			return
		}
		// Pre-parse the splice as inline markdown. The splice is a
		// single-paragraph fragment; we extract its inlines for direct
		// substitution into the AST.
		replacements[key] = r.parseInlineSplice(val)
	})
	if validationErr != nil {
		return nil, validationErr
	}
	return replacements, nil
}

// parseInlineSplice parses a markdown fragment as inline content and
// returns the resulting `inlines` array. If the fragment doesn't parse
// to a single paragraph, returns a single text inline as a fallback so
// the splice never silently disappears.
func (r *Runtime) parseInlineSplice(s string) *lua.LTable {
	source := []byte(s)
	md := goldmark.New(goldmark.WithExtensions(
		extension.NewTable(),
		extension.TaskList,
		extension.Strikethrough,
	))
	doc := md.Parser().Parse(text.NewReader(source))
	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		if child.Kind() == ast.KindParagraph {
			return r.extractInlines(child, source)
		}
	}
	// Fallback: wrap as a single text inline.
	out := r.L.NewTable()
	out.RawSetInt(1, r.makeLeafInline(inlineTypeText, s))
	return out
}

// rewriteCodeSpanRefs walks every block in the AST and rewrites
// code-span inlines whose text matches a key in replacements.
func (r *Runtime) rewriteCodeSpanRefs(nodes *lua.LTable, replacements map[string]*lua.LTable) {
	for i := 1; i <= nodes.Len(); i++ {
		node, ok := nodes.RawGetInt(i).(*lua.LTable)
		if !ok {
			continue
		}
		r.rewriteCodeSpanRefsInBlock(node, replacements)
	}
}

// rewriteCodeSpanRefsInBlock dispatches on block kind: paragraph,
// heading, blockquote, list, table. Each visits the inline arrays
// hanging off the block.
func (r *Runtime) rewriteCodeSpanRefsInBlock(node *lua.LTable, replacements map[string]*lua.LTable) {
	switch blockKindOf(node) {
	case nodeTypeParagraph, nodeTypeHeading:
		if t := inlinesField(node, "inlines"); t != nil {
			r.rewriteCodeSpansInInlines(node, "inlines", t, replacements)
		}
	case nodeTypeBlockquote:
		if children := inlinesField(node, "children"); children != nil {
			r.rewriteCodeSpanRefs(children, replacements)
		}
	case nodeTypeList:
		r.rewriteCodeSpanRefsInList(node, replacements)
	case nodeTypeTable:
		r.rewriteCodeSpanRefsInTable(node, replacements)
	default:
		// code_block, thematic_break, raw — no inline content to rewrite.
	}
}

func (r *Runtime) rewriteCodeSpanRefsInList(node *lua.LTable, replacements map[string]*lua.LTable) {
	items := inlinesField(node, "items")
	if items == nil {
		return
	}
	for i := 1; i <= items.Len(); i++ {
		item, ok := items.RawGetInt(i).(*lua.LTable)
		if !ok {
			// Plain string list items contain only text — no code spans
			// to rewrite.
			continue
		}
		if t := inlinesField(item, "inlines"); t != nil {
			r.rewriteCodeSpansInInlines(item, "inlines", t, replacements)
		}
		if children := inlinesField(item, "children"); children != nil {
			r.rewriteCodeSpanRefs(children, replacements)
		}
	}
}

func (r *Runtime) rewriteCodeSpanRefsInTable(node *lua.LTable, replacements map[string]*lua.LTable) {
	if header := inlinesField(node, "header"); header != nil {
		for i := 1; i <= header.Len(); i++ {
			if cell, ok := header.RawGetInt(i).(*lua.LTable); ok {
				r.rewriteCodeSpansInTableCell(header, i, cell, replacements)
			}
		}
	}
	rows := inlinesField(node, "rows")
	if rows == nil {
		return
	}
	for ri := 1; ri <= rows.Len(); ri++ {
		row, ok := rows.RawGetInt(ri).(*lua.LTable)
		if !ok {
			continue
		}
		for ci := 1; ci <= row.Len(); ci++ {
			if cell, ok := row.RawGetInt(ci).(*lua.LTable); ok {
				r.rewriteCodeSpansInTableCell(row, ci, cell, replacements)
			}
		}
	}
}

// rewriteCodeSpansInInlines rewrites code-span entries in a parent
// inlines array. The walk is in-place: matched code spans are spliced
// out and replaced with the (possibly multi-element) inlines from the
// replacement value. Container inlines (link, emphasis, strong,
// strikethrough) are recursed into.
func (r *Runtime) rewriteCodeSpansInInlines(
	parent *lua.LTable, field string, inlines *lua.LTable,
	replacements map[string]*lua.LTable,
) {
	out := r.L.NewTable()
	idx := 1
	for i := 1; i <= inlines.Len(); i++ {
		v := inlines.RawGetInt(i)
		child, ok := v.(*lua.LTable)
		if !ok {
			out.RawSetInt(idx, v)
			idx++
			continue
		}
		switch inlineKindOf(child) {
		case inlineTypeCodeSpan:
			text, _ := child.RawGetString("text").(lua.LString)
			if splice, hit := replacements[string(text)]; hit {
				// Append cloned splice inlines.
				for j := 1; j <= splice.Len(); j++ {
					if sn, ok := splice.RawGetInt(j).(*lua.LTable); ok {
						out.RawSetInt(idx, r.deepCopyTable(sn))
						idx++
					}
				}
				continue
			}
			out.RawSetInt(idx, child)
			idx++
		case inlineTypeLink, inlineTypeEmphasis, inlineTypeStrong, inlineTypeStrikethrough:
			if t := inlinesField(child, "inlines"); t != nil {
				r.rewriteCodeSpansInInlines(child, "inlines", t, replacements)
			}
			out.RawSetInt(idx, child)
			idx++
		case inlineTypeImage:
			if t := inlinesField(child, "alt_inlines"); t != nil {
				r.rewriteCodeSpansInInlines(child, "alt_inlines", t, replacements)
			}
			out.RawSetInt(idx, child)
			idx++
		default:
			out.RawSetInt(idx, child)
			idx++
		}
	}
	parent.RawSetString(field, out)
}

// rewriteCodeSpansInTableCell handles a single table cell. Cells live in
// an int-indexed slot on their row table; the helper builds a fresh
// inlines array and re-attaches it via RawSetInt.
func (r *Runtime) rewriteCodeSpansInTableCell(
	parent *lua.LTable, idx int, cell *lua.LTable,
	replacements map[string]*lua.LTable,
) {
	out := r.L.NewTable()
	pos := 1
	for i := 1; i <= cell.Len(); i++ {
		v := cell.RawGetInt(i)
		child, ok := v.(*lua.LTable)
		if !ok {
			out.RawSetInt(pos, v)
			pos++
			continue
		}
		if inlineKindOf(child) == inlineTypeCodeSpan {
			text, _ := child.RawGetString("text").(lua.LString)
			if splice, hit := replacements[string(text)]; hit {
				for j := 1; j <= splice.Len(); j++ {
					if sn, ok := splice.RawGetInt(j).(*lua.LTable); ok {
						out.RawSetInt(pos, r.deepCopyTable(sn))
						pos++
					}
				}
				continue
			}
		}
		// Recurse for containers.
		switch inlineKindOf(child) {
		case inlineTypeLink, inlineTypeEmphasis, inlineTypeStrong, inlineTypeStrikethrough:
			if t := inlinesField(child, "inlines"); t != nil {
				r.rewriteCodeSpansInInlines(child, "inlines", t, replacements)
			}
		default:
			// Other inline kinds (text, raw_html, autolink, breaks,
			// image, code_span — already handled above) have no nested
			// inlines we need to recurse into here.
		}
		out.RawSetInt(pos, child)
		pos++
	}
	parent.RawSetInt(idx, out)
}

// deepCopyAST deep-copies the top-level array of node tables. Non-table
// entries are passed through (defensive — should not normally occur).
func (r *Runtime) deepCopyAST(astTable *lua.LTable) *lua.LTable {
	out := r.L.NewTable()
	for i := 1; i <= astTable.Len(); i++ {
		v := astTable.RawGetInt(i)
		if t, ok := v.(*lua.LTable); ok {
			out.RawSetInt(i, r.deepCopyNode(t))
		} else {
			out.RawSetInt(i, v)
		}
	}
	return out
}

// --- entity_refs ---

// luaMdEntityRefs implements rela.md.entity_refs(opts?) -> {[id] = string}.
//
// Builds a replacement map covering some or all entities in the project
// for use with resolve_refs. Iterates the store per entity type because
// store.ListEntities requires a non-empty Type.
func (r *Runtime) luaMdEntityRefs(ls *lua.LState) int {
	if r.deps.Meta == nil || r.deps.Store == nil {
		ls.RaiseError("rela.md.entity_refs: requires a runtime with metamodel and store")
		return 0
	}
	opts, err := r.parseEntityRefsOpts(ls)
	if err != nil {
		ls.RaiseError("%s", err.Error())
		return 0
	}
	typeNames := opts.types
	if len(typeNames) == 0 && opts.useAllTypes {
		typeNames = r.deps.Meta.EntityTypes()
	}

	out := r.L.NewTable()
	ctx := context.Background()
	for _, t := range typeNames {
		for e, listErr := range r.deps.Store.ListEntities(ctx, store.EntityQuery{Type: t}) {
			if listErr != nil {
				ls.RaiseError(
					"rela.md.entity_refs: list entities of type %q: %s", t, listErr.Error())
				return 0
			}
			value, valErr := r.buildEntityRefValue(ls, e, opts)
			if valErr != nil {
				ls.RaiseError("%s", valErr.Error())
				return 0
			}
			out.RawSetString(e.ID, lua.LString(value))
		}
	}
	ls.Push(out)
	return 1
}

type entityRefsOpts struct {
	types       []string
	useAllTypes bool
	style       string
	formatFn    *lua.LFunction
}

func (r *Runtime) parseEntityRefsOpts(ls *lua.LState) (entityRefsOpts, error) {
	opts := entityRefsOpts{style: "title-slug", useAllTypes: true}
	if ls.GetTop() < 1 || ls.Get(1) == lua.LNil {
		return opts, nil
	}
	t := ls.CheckTable(1)

	if v := t.RawGetString("types"); v != lua.LNil {
		tbl, ok := v.(*lua.LTable)
		if !ok {
			return opts, errors.New("rela.md.entity_refs: opts.types must be a table")
		}
		opts.useAllTypes = false
		for i := 1; i <= tbl.Len(); i++ {
			name, ok := tbl.RawGetInt(i).(lua.LString)
			if !ok {
				return opts, errors.New("rela.md.entity_refs: opts.types must contain only strings")
			}
			if _, found := r.deps.Meta.GetEntityDef(string(name)); !found {
				return opts, fmt.Errorf("rela.md.entity_refs: unknown entity type %q", string(name))
			}
			opts.types = append(opts.types, string(name))
		}
	}

	if v := t.RawGetString("style"); v != lua.LNil {
		s, ok := v.(lua.LString)
		if !ok {
			return opts, errors.New("rela.md.entity_refs: opts.style must be a string")
		}
		opts.style = string(s)
		if opts.style != "title-slug" && opts.style != "id" {
			return opts, fmt.Errorf(
				"rela.md.entity_refs: opts.style must be \"title-slug\" or \"id\", got %q", opts.style)
		}
	}

	if v := t.RawGetString("format"); v != lua.LNil {
		fn, ok := v.(*lua.LFunction)
		if !ok {
			return opts, errors.New("rela.md.entity_refs: opts.format must be a function")
		}
		opts.formatFn = fn
	}

	return opts, nil
}

func (r *Runtime) buildEntityRefValue(ls *lua.LState, e *entity.Entity, opts entityRefsOpts) (string, error) {
	if opts.formatFn != nil {
		entityTbl := EntityToTable(ls, e)
		entityTbl.RawSetString("title", lua.LString(e.Title()))
		ls.Push(opts.formatFn)
		ls.Push(entityTbl)
		if perr := ls.PCall(1, 1, nil); perr != nil {
			return "", fmt.Errorf("rela.md.entity_refs: format callback failed: %s", perr.Error())
		}
		ret := ls.Get(-1)
		ls.Pop(1)
		rs, ok := ret.(lua.LString)
		if !ok {
			return "", fmt.Errorf(
				"rela.md.entity_refs: format callback must return a string for entity %q", e.ID)
		}
		value := string(rs)
		if strings.ContainsAny(value, "\n\r") {
			return "", fmt.Errorf(
				"rela.md.entity_refs: format callback returned a newline for entity %q", e.ID)
		}
		return value, nil
	}
	title := escapeMarkdownLinkText(e.Title())
	anchor := titleSlug(e.Title())
	if opts.style == "id" {
		anchor = strings.ToLower(e.ID)
	}
	return "[" + title + "](#" + anchor + ")", nil
}

// titleSlug derives a Pandoc-style anchor from a title: lowercase, runs
// of non-alphanumeric Unicode collapsed to '-', leading/trailing '-'
// trimmed. Letters and digits include non-ASCII so anchors line up with
// auto-heading-id renderers (Pandoc, goldmark, markdown-it-anchor).
func titleSlug(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevDash := true
	for _, r := range strings.ToLower(s) {
		if isSlugLetterOrDigit(r) {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

func isSlugLetterOrDigit(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// escapeMarkdownLinkText escapes characters that would break out of a
// markdown link's text slot.
func escapeMarkdownLinkText(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`)
	return r.Replace(s)
}
