// Package lua provides markdown AST manipulation functions for Lua scripts.
// The rela.md module enables parsing, transforming, and rendering markdown content.
package lua

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	lua "github.com/yuin/gopher-lua"
)

// Markdown node type constants.
const (
	nodeTypeHeading       = "heading"
	nodeTypeParagraph     = "paragraph"
	nodeTypeCodeBlock     = "code_block"
	nodeTypeList          = "list"
	nodeTypeBlockquote    = "blockquote"
	nodeTypeThematicBreak = "thematic_break"
	nodeTypeTable         = "table"
	nodeTypeRaw           = "raw"

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

	// Node constructors
	r.L.SetField(md, "heading", r.L.NewFunction(luaMdHeading))
	r.L.SetField(md, "paragraph", r.L.NewFunction(luaMdParagraph))
	r.L.SetField(md, "code_block", r.L.NewFunction(luaMdCodeBlock))
	r.L.SetField(md, "thematic_break", r.L.NewFunction(luaMdThematicBreak))
	r.L.SetField(md, "blockquote", r.L.NewFunction(luaMdBlockquote))
	r.L.SetField(md, "list", r.L.NewFunction(luaMdList))

	// Generation helpers
	r.L.SetField(md, "link", r.L.NewFunction(luaMdLink))
	r.L.SetField(md, "ref", r.L.NewFunction(luaMdRef))
	r.L.SetField(md, "table", r.L.NewFunction(luaMdTable))
	r.L.SetField(md, "entity_table", r.L.NewFunction(r.luaMdEntityTable))

	r.L.SetField(rela, "md", md)
}

// --- Core Functions ---

// luaMdParse parses markdown content into an AST table.
// Usage: local ast = rela.md.parse(content)
func (r *Runtime) luaMdParse(ls *lua.LState) int {
	content := ls.CheckString(1)

	source := []byte(content)
	md := goldmark.New(goldmark.WithExtensions(extension.NewTable()))
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
	r.renderNodes(&sb, astTable)

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
			header.RawSetString("title", node.RawGetString("text"))
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

// getHeadingInfo extracts level and title from a heading node.
func (r *Runtime) getHeadingInfo(node *lua.LTable) (level int, title string) {
	level = 1
	if l, ok := node.RawGetString("level").(lua.LNumber); ok {
		level = int(l)
	}
	if t, ok := node.RawGetString("text").(lua.LString); ok {
		title = string(t)
	}
	return
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
		if nodeType := node.RawGetString("type"); nodeType == lua.LString(nodeTypeParagraph) {
			ls.Push(node.RawGetString("text"))
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
func luaMdHeading(ls *lua.LState) int {
	level := ls.CheckInt(1)
	headingText := ls.CheckString(2)

	// Clamp level
	if level < minHeaderLevel {
		level = minHeaderLevel
	}
	if level > maxHeaderLevel {
		level = maxHeaderLevel
	}

	node := ls.NewTable()
	node.RawSetString("type", lua.LString(nodeTypeHeading))
	node.RawSetString("level", lua.LNumber(level))
	node.RawSetString("text", lua.LString(headingText))

	ls.Push(node)
	return 1
}

// luaMdParagraph creates a paragraph node.
// Usage: local node = rela.md.paragraph("Some text content")
func luaMdParagraph(ls *lua.LState) int {
	paragraphText := ls.CheckString(1)

	node := ls.NewTable()
	node.RawSetString("type", lua.LString(nodeTypeParagraph))
	node.RawSetString("text", lua.LString(paragraphText))

	ls.Push(node)
	return 1
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
func luaMdBlockquote(ls *lua.LState) int {
	content := ls.CheckString(1)

	node := ls.NewTable()
	node.RawSetString("type", lua.LString(nodeTypeBlockquote))
	node.RawSetString("content", lua.LString(content))

	ls.Push(node)
	return 1
}

// luaMdList creates a list node.
// Usage: local node = rela.md.list({"item1", "item2"}, false)
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
		node.RawSetString("text", lua.LString(r.extractText(n, source)))

	case *ast.Paragraph:
		node.RawSetString("type", lua.LString(nodeTypeParagraph))
		node.RawSetString("text", lua.LString(r.extractText(n, source)))

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
		node.RawSetString("content", lua.LString(r.extractBlockquoteContent(n, source)))

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

// extractText extracts text content from inline children.
func (r *Runtime) extractText(n ast.Node, source []byte) string {
	var sb strings.Builder
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		r.extractInlineText(&sb, child, source)
	}
	return sb.String()
}

// extractInlineText recursively extracts text from inline nodes.
func (r *Runtime) extractInlineText(sb *strings.Builder, n ast.Node, source []byte) {
	switch n := n.(type) {
	case *ast.Text:
		sb.Write(n.Segment.Value(source))
		if n.SoftLineBreak() {
			sb.WriteByte(' ')
		}
	case *ast.String:
		sb.Write(n.Value)
	default:
		// Recurse into children for other inline types (emphasis, links, etc.)
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			r.extractInlineText(sb, child, source)
		}
	}
}

// extractCodeBlockContent extracts content from a fenced code block.
func (r *Runtime) extractCodeBlockContent(fcb *ast.FencedCodeBlock, source []byte) string {
	var sb strings.Builder
	lines := fcb.Lines()
	for i := 0; i < lines.Len(); i++ {
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
		for i := 0; i < lines.Len(); i++ {
			line := lines.At(i)
			sb.Write(line.Value(source))
		}
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

// extractListItems extracts list items as a Lua table.
func (r *Runtime) extractListItems(n ast.Node, source []byte) *lua.LTable {
	items := r.L.NewTable()
	idx := 1
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if child.Kind() == ast.KindListItem {
			// Extract text from the list item's children
			var sb strings.Builder
			for itemChild := child.FirstChild(); itemChild != nil; itemChild = itemChild.NextSibling() {
				if itemChild.Kind() == ast.KindTextBlock || itemChild.Kind() == ast.KindParagraph {
					sb.WriteString(r.extractText(itemChild, source))
				}
			}
			items.RawSetInt(idx, lua.LString(sb.String()))
			idx++
		}
	}
	return items
}

// extractBlockquoteContent extracts text from a blockquote.
func (r *Runtime) extractBlockquoteContent(n ast.Node, source []byte) string {
	var sb strings.Builder
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if child.Kind() == ast.KindParagraph {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(r.extractText(child, source))
		}
	}
	return sb.String()
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

// extractTableData extracts header, rows, and alignments from a GFM table node.
// Note: cell text is extracted as plain text via extractText(), which strips inline
// formatting (bold, links, code spans). This is consistent with heading/paragraph
// extraction but means rich inline content in cells is not preserved.
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

	// Walk children: TableHeader contains the header row, TableRows are data rows
	rowIdx := 1
	for child := table.FirstChild(); child != nil; child = child.NextSibling() {
		switch child.Kind() {
		case east.KindTableHeader:
			// Header has TableCell children directly
			cellIdx := 1
			for cell := child.FirstChild(); cell != nil; cell = cell.NextSibling() {
				header.RawSetInt(cellIdx, lua.LString(r.extractText(cell, source)))
				cellIdx++
			}
		case east.KindTableRow:
			// Data row with cells
			luaRow := r.L.NewTable()
			cellIdx := 1
			for cell := child.FirstChild(); cell != nil; cell = cell.NextSibling() {
				luaRow.RawSetInt(cellIdx, lua.LString(r.extractText(cell, source)))
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
func (r *Runtime) renderNodes(sb *strings.Builder, nodes *lua.LTable) {
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
		r.renderNode(sb, node)
	}
}

// renderNode renders a single AST node to markdown.
func (r *Runtime) renderNode(sb *strings.Builder, node *lua.LTable) {
	nodeType, _ := node.RawGetString("type").(lua.LString)

	switch string(nodeType) {
	case nodeTypeHeading:
		r.renderHeading(sb, node)
	case nodeTypeParagraph:
		r.renderParagraph(sb, node)
	case nodeTypeCodeBlock:
		r.renderCodeBlock(sb, node)
	case nodeTypeList:
		r.renderList(sb, node)
	case nodeTypeBlockquote:
		r.renderBlockquote(sb, node)
	case nodeTypeThematicBreak:
		sb.WriteString("---\n")
	case nodeTypeTable:
		r.renderTableNode(sb, node)
	case nodeTypeRaw:
		r.renderRaw(sb, node)
	}
}

// renderHeading renders a heading node.
func (r *Runtime) renderHeading(sb *strings.Builder, node *lua.LTable) {
	level := 1
	if l, ok := node.RawGetString("level").(lua.LNumber); ok {
		level = int(l)
	}
	title := ""
	if t, ok := node.RawGetString("text").(lua.LString); ok {
		title = string(t)
	}
	sb.WriteString(strings.Repeat("#", level))
	sb.WriteString(" ")
	sb.WriteString(title)
	sb.WriteString("\n")
}

// renderParagraph renders a paragraph node.
func (r *Runtime) renderParagraph(sb *strings.Builder, node *lua.LTable) {
	content := ""
	if t, ok := node.RawGetString("text").(lua.LString); ok {
		content = string(t)
	}
	sb.WriteString(content)
	sb.WriteString("\n")
}

// renderCodeBlock renders a code block node.
func (r *Runtime) renderCodeBlock(sb *strings.Builder, node *lua.LTable) {
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

// renderList renders a list node.
func (r *Runtime) renderList(sb *strings.Builder, node *lua.LTable) {
	ordered := false
	if o, ok := node.RawGetString("ordered").(lua.LBool); ok {
		ordered = bool(o)
	}
	items, ok := node.RawGetString("items").(*lua.LTable)
	if !ok {
		return
	}
	// Use sequential access to preserve order
	for i := 1; i <= items.Len(); i++ {
		v := items.RawGetInt(i)
		s, ok := v.(lua.LString)
		if !ok {
			continue
		}
		if ordered {
			fmt.Fprintf(sb, "%d. ", i)
		} else {
			sb.WriteString("- ")
		}
		sb.WriteString(string(s))
		sb.WriteString("\n")
	}
}

// renderBlockquote renders a blockquote node.
func (r *Runtime) renderBlockquote(sb *strings.Builder, node *lua.LTable) {
	content := ""
	if c, ok := node.RawGetString("content").(lua.LString); ok {
		content = string(c)
	}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		sb.WriteString("> ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
}

// renderRaw renders a raw node.
func (r *Runtime) renderRaw(sb *strings.Builder, node *lua.LTable) {
	content := ""
	if c, ok := node.RawGetString("content").(lua.LString); ok {
		content = string(c)
	}
	sb.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		sb.WriteString("\n")
	}
}

// renderTableNode renders a table AST node to GFM markdown with column-aligned padding.
func (r *Runtime) renderTableNode(sb *strings.Builder, node *lua.LTable) {
	header, _ := node.RawGetString("header").(*lua.LTable)
	rows, _ := node.RawGetString("rows").(*lua.LTable)
	alignments, _ := node.RawGetString("alignments").(*lua.LTable)

	if header == nil || header.Len() == 0 {
		return
	}

	numCols := header.Len()

	// Collect all cell values and compute column widths
	headerCells := make([]string, numCols)
	colWidths := make([]int, numCols)
	colAligns := make([]string, numCols)

	for i := 0; i < numCols; i++ {
		headerCells[i] = lua.LVAsString(header.RawGetInt(i + 1))
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
			for j := 0; j < numCols; j++ {
				cells[j] = lua.LVAsString(row.RawGetInt(j + 1))
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
	for i := 0; i < numCols; i++ {
		sb.WriteString(" ")
		sb.WriteString(padCell(headerCells[i], colWidths[i], colAligns[i]))
		sb.WriteString(" |")
	}
	sb.WriteString("\n")

	// Separator row
	sb.WriteString("|")
	for i := 0; i < numCols; i++ {
		sb.WriteString(" ")
		sb.WriteString(renderSeparator(colWidths[i], colAligns[i]))
		sb.WriteString(" |")
	}
	sb.WriteString("\n")

	// Data rows
	for _, cells := range dataRows {
		sb.WriteString("|")
		for j := 0; j < numCols; j++ {
			sb.WriteString(" ")
			sb.WriteString(padCell(cells[j], colWidths[j], colAligns[j]))
			sb.WriteString(" |")
		}
		sb.WriteString("\n")
	}
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
