// Lua bindings for the rela.xml.* submodule.
//
//	rela.xml.decode(string)          -> (node, nil) | (nil, err_table)
//	rela.xml.find(node, name[, ns])  -> first child element named name, or nil
//	rela.xml.find_all(node, name[, ns]) -> list of child elements named name
//	rela.xml.text(node)              -> all character data inside node
//
// decode goes through internal/safexml, so untrusted XML is decoded under
// fixed limits (size, depth, element and attribute counts, charset
// allowlist) with no DTD processing and no external entities. The safety
// argument lives in that package's doc; this file only shapes the result.
//
// A node is a plain table:
//
//	name     local element name
//	ns       namespace URI (absent when the element has none)
//	attrs    map of attribute values (absent when there are none); an
//	         unqualified attribute is keyed by its name, a qualified one by
//	         "{uri}name". Namespace declarations are not attributes.
//	children list of child nodes (absent when there are none)
//	text     character data before the first child
//	tail     character data after this element, before the next sibling
//
// text/tail is the ElementTree model: it keeps mixed content in order.
// rela.xml.text(node) joins all of it for the common "give me the string"
// case. Empty fields are left out to keep a large document's table count
// down.
//
// Error convention follows rela.json.decode: invalid or refused input
// returns (nil, err_table) with kind="bad_response"; a wrong argument type
// raises. When the input was refused for exceeding a limit, err.limit names
// it ("bytes", "depth", "elements", "attributes", "charset", or "items" from
// rela.feed.parse).
package lua

import (
	"errors"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/safexml"
)

// registerXMLModule adds the rela.xml submodule to the rela table. A free
// function, like registerCryptoModule, to keep the Runtime method count flat.
func registerXMLModule(ls *lua.LState, rela *lua.LTable) {
	x := ls.NewTable()
	ls.SetField(x, "decode", ls.NewFunction(luaXMLDecode))
	ls.SetField(x, "find", ls.NewFunction(luaXMLFind))
	ls.SetField(x, "find_all", ls.NewFunction(luaXMLFindAll))
	ls.SetField(x, "text", ls.NewFunction(luaXMLText))
	ls.SetField(rela, "xml", x)
}

func luaXMLDecode(ls *lua.LState) int {
	str := ls.CheckString(1)
	root, err := safexml.Decode([]byte(str), safexml.DefaultLimits)
	if err != nil {
		return pushDecodeError(ls, "xml.decode", err)
	}
	ls.Push(xmlNodeToLua(ls, root))
	ls.Push(lua.LNil)
	return 2
}

// pushDecodeError pushes (nil, err_table) for a failed decode in the shape
// rela.json.decode uses, plus the limit field for a refused input.
func pushDecodeError(ls *lua.LState, fn string, err error) int {
	ls.Push(lua.LNil)
	tbl := ls.NewTable()
	tbl.RawSetString("kind", lua.LString("bad_response"))
	tbl.RawSetString("status", lua.LNumber(0))
	tbl.RawSetString("message", lua.LString(fn+": "+err.Error()))
	tbl.RawSetString("retry_after", lua.LNumber(0))
	tbl.RawSetString("details", lua.LString(err.Error()))
	if le, ok := errors.AsType[*safexml.LimitError](err); ok {
		tbl.RawSetString("limit", lua.LString(le.Limit))
	}
	ls.Push(tbl)
	return 2
}

// xmlNodeToLua converts a decoded tree. Recursion depth is bounded by
// safexml's depth limit.
func xmlNodeToLua(ls *lua.LState, n *safexml.Node) *lua.LTable {
	t := ls.NewTable()
	t.RawSetString("name", lua.LString(n.Name))
	if n.Space != "" {
		t.RawSetString("ns", lua.LString(n.Space))
	}
	if len(n.Attrs) > 0 {
		attrs := ls.NewTable()
		for _, a := range n.Attrs {
			key := a.Name
			if a.Space != "" {
				key = "{" + a.Space + "}" + a.Name
			}
			attrs.RawSetString(key, lua.LString(a.Value))
		}
		t.RawSetString("attrs", attrs)
	}
	if len(n.Children) > 0 {
		children := ls.CreateTable(len(n.Children), 0)
		for i, c := range n.Children {
			children.RawSetInt(i+1, xmlNodeToLua(ls, c))
		}
		t.RawSetString("children", children)
	}
	if n.Text != "" {
		t.RawSetString("text", lua.LString(n.Text))
	}
	if n.Tail != "" {
		t.RawSetString("tail", lua.LString(n.Tail))
	}
	return t
}

// xmlChildren calls fn for each child element of the node table at
// argument 1 whose name (and namespace, when argument 3 is given) match.
// fn returns false to stop.
func xmlChildren(ls *lua.LState, fn func(*lua.LTable) bool) {
	node := ls.CheckTable(1)
	name := ls.CheckString(2)
	ns, wantNS := "", ls.GetTop() >= 3 && ls.Get(3) != lua.LNil
	if wantNS {
		ns = ls.CheckString(3)
	}
	children, ok := node.RawGetString("children").(*lua.LTable)
	if !ok {
		return
	}
	for i := 1; i <= children.Len(); i++ {
		c, ok := children.RawGetInt(i).(*lua.LTable)
		if !ok || lua.LVAsString(c.RawGetString("name")) != name {
			continue
		}
		if wantNS && lua.LVAsString(c.RawGetString("ns")) != ns {
			continue
		}
		if !fn(c) {
			return
		}
	}
}

func luaXMLFind(ls *lua.LState) int {
	found := lua.LNil
	xmlChildren(ls, func(c *lua.LTable) bool {
		found = c
		return false
	})
	ls.Push(found)
	return 1
}

func luaXMLFindAll(ls *lua.LState) int {
	out := ls.NewTable()
	xmlChildren(ls, func(c *lua.LTable) bool {
		out.Append(c)
		return true
	})
	ls.Push(out)
	return 1
}

func luaXMLText(ls *lua.LState) int {
	w := textWalk{budget: safexml.DefaultLimits.MaxElements}
	w.walk(ls.CheckTable(1), 0)
	ls.Push(lua.LString(w.b.String()))
	return 1
}

// textWalk joins the text of a node table. A script can hand-build the table,
// including cycles and nodes shared many times over, so neither depth nor
// shape bounds the work: a table whose children all point back at it would
// take exponential time. The walk therefore stops after visiting as many
// nodes as safexml would ever produce, and after as many bytes as it would
// ever accept.
type textWalk struct {
	b      strings.Builder
	budget int
}

func (w *textWalk) done() bool {
	return w.budget <= 0 || w.b.Len() >= safexml.DefaultLimits.MaxBytes
}

func (w *textWalk) walk(node *lua.LTable, depth int) {
	if depth >= safexml.DefaultLimits.MaxDepth || w.done() {
		return
	}
	w.budget--
	w.b.WriteString(lua.LVAsString(node.RawGetString("text")))
	children, ok := node.RawGetString("children").(*lua.LTable)
	if !ok {
		return
	}
	for i := 1; i <= children.Len() && !w.done(); i++ {
		if c, ok := children.RawGetInt(i).(*lua.LTable); ok {
			w.walk(c, depth+1)
			w.b.WriteString(lua.LVAsString(c.RawGetString("tail")))
		}
	}
}
