package lua

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// runWithInput runs code in a reader runtime with the global `input` set to
// in. A reader runtime is the most restricted kind, so passing there also
// pins that the modules need no capability.
func runWithInput(t *testing.T, in, code string) {
	t.Helper()
	r := newTestRuntime(t)
	defer r.Close()
	r.L.SetGlobal("input", lua.LString(in))
	if err := r.RunString(code); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaXML_Decode(t *testing.T) {
	t.Parallel()
	doc := `<?xml version="1.0"?>
<root xmlns="urn:r" xmlns:x="urn:x" id="7" x:kind="k">
  <item>one</item>
  <item>two <b>bold</b> tail</item>
  <x:other/>
</root>`
	runWithInput(t, doc, `
		local root, err = rela.xml.decode(input)
		assert(err == nil, err and err.message)
		assert(root.name == "root" and root.ns == "urn:r", "root name/ns")
		assert(root.attrs.id == "7", "unqualified attr")
		assert(root.attrs["{urn:x}kind"] == "k", "qualified attr")
		assert(root.attrs.xmlns == nil, "namespace declarations are not attributes")
		assert(#root.children == 3, "children: " .. #root.children)

		local items = rela.xml.find_all(root, "item")
		assert(#items == 2, "find_all")
		assert(items[1].text == "one")
		assert(items[1].children == nil, "empty children omitted")
		assert(items[2].text == "two ", "text before first child")
		assert(items[2].children[1].tail == " tail", "tail")
		assert(rela.xml.text(items[2]) == "two bold tail", "text joins mixed content")

		assert(rela.xml.find(root, "other", "urn:x") ~= nil, "find with ns")
		assert(rela.xml.find(root, "other", "urn:r") == nil, "ns mismatch")
		assert(rela.xml.find(root, "missing") == nil, "missing child")
		assert(rela.xml.find(items[1], "item") == nil, "leaf has no children")
		assert(#rela.xml.find_all(root, "missing") == 0)
	`)
}

func TestLuaXML_DecodeErrors(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, doc, check string }{
		{"malformed", "<a>", `err.limit == nil and err.message:find("xml.decode", 1, true)`},
		{"entity bomb", `<!DOCTYPE a [<!ENTITY e "x">]><a>&e;</a>`, `err.limit == nil`},
		{"too deep", strings.Repeat("<a>", 65) + strings.Repeat("</a>", 65), `err.limit == "depth"`},
		{"charset", `<?xml version="1.0" encoding="utf-16"?><a/>`, `err.limit == "charset"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runWithInput(t, tc.doc, `
				local v, err = rela.xml.decode(input)
				assert(v == nil and err ~= nil, "want error")
				assert(err.kind == "bad_response", "kind")
				assert(`+tc.check+`, "check failed: " .. tostring(err.limit) .. " " .. err.message)
			`)
		})
	}
}

func TestLuaXML_WrongArgumentRaises(t *testing.T) {
	t.Parallel()
	r := newTestRuntime(t)
	defer r.Close()
	for _, code := range []string{
		`rela.xml.decode({})`,
		`rela.xml.find("x", "a")`,
		`rela.xml.find({}, nil)`,
		`rela.xml.text(1)`,
	} {
		if err := r.RunString(code); err == nil {
			t.Errorf("%s: want error", code)
		}
	}
}

func TestLuaXML_TextStopsOnHandBuiltCycle(t *testing.T) {
	t.Parallel()
	runWithInput(t, "", `
		local n = {text = "x"}
		n.children = {n}
		local s = rela.xml.text(n)
		assert(#s > 0 and #s <= 64, "bounded walk, got length " .. #s)

		-- Ten children that are all the node itself: without a node budget
		-- this is 10^64 visits.
		local w = {text = "y", children = {}}
		for i = 1, 10 do w.children[i] = w end
		s = rela.xml.text(w)
		assert(#s <= 100000, "budgeted walk, got length " .. #s)
	`)
}

func TestLuaFeed_Parse(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "feed", "testdata", "rss2.xml"))
	if err != nil {
		t.Fatal(err)
	}
	runWithInput(t, string(data), `
		local f, err = rela.feed.parse(input)
		assert(err == nil, err and err.message)
		assert(f.format == "rss" and f.title == "Example & Co")
		assert(#f.items == 3)

		local a = f.items[1]
		assert(a.id == "urn:post:1")
		assert(a.content == "<p>Full <b>body</b></p>")
		assert(a.published == "2026-10-13T14:30:00Z", tostring(a.published))
		assert(a.published_raw == "Tue, 13 Oct 2026 09:30:00 EST")
		assert(a.updated == nil and a.updated_raw == nil)
		assert(#a.authors == 2 and a.authors[2] == "Bob")
		assert(#a.categories == 2)

		local b = f.items[2]
		assert(b.published == nil and b.published_raw == "not a date")

		local c = f.items[3]
		assert(c.id == nil, "no identity is invented")
		assert(c.title == "" and c.link == "")
		assert(#c.authors == 0)
	`)
}

func TestLuaFeed_ParseErrors(t *testing.T) {
	t.Parallel()
	runWithInput(t, "<html/>", `
		local f, err = rela.feed.parse(input)
		assert(f == nil and err.kind == "bad_response")
		assert(err.message:find("feed.parse", 1, true))
		assert(err.message:find("unrecognized root", 1, true))
		assert(err.limit == nil)
	`)
	runWithInput(t, "<rss>"+strings.Repeat("<a>", 70)+strings.Repeat("</a>", 70)+"</rss>", `
		local f, err = rela.feed.parse(input)
		assert(f == nil and err.limit == "depth")
	`)
}
