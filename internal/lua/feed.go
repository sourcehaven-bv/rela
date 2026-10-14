// Lua bindings for the rela.feed.* submodule.
//
//	rela.feed.parse(string) -> (feed, nil) | (nil, err_table)
//
// Parses RSS 2.0 / 0.9x, RSS 1.0, Atom 1.0 and JSON Feed through
// internal/feed, the parser the declarative feed import uses too, so a
// script and a subscription read the same feed the same way. XML goes
// through internal/safexml's limits.
//
// The result:
//
//	format       "rss" | "rdf" | "atom" | "json"
//	title, link, description
//	items        list of:
//	  id            the source's identifier, else the link; nil when neither
//	  title, link   strings ("" when absent)
//	  summary, content  HTML strings ("" when absent); untrusted, so convert
//	                with rela.md.from_html rather than rendering them
//	  published, updated  RFC 3339 UTC strings, nil when absent or unparseable
//	  published_raw, updated_raw  the source text, nil when absent
//	  authors, categories  lists of strings
//
// Errors follow rela.xml.decode: (nil, err_table) with kind="bad_response",
// and err.limit set when a limit refused the input.
package lua

import (
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/feed"
)

// registerFeedModule adds the rela.feed submodule to the rela table.
func registerFeedModule(ls *lua.LState, rela *lua.LTable) {
	f := ls.NewTable()
	ls.SetField(f, "parse", ls.NewFunction(luaFeedParse))
	ls.SetField(rela, "feed", f)
}

func luaFeedParse(ls *lua.LState) int {
	str := ls.CheckString(1)
	parsed, err := feed.Parse([]byte(str))
	if err != nil {
		return pushDecodeError(ls, "feed.parse", err)
	}
	t := ls.NewTable()
	t.RawSetString("format", lua.LString(parsed.Format))
	t.RawSetString("title", lua.LString(parsed.Title))
	t.RawSetString("link", lua.LString(parsed.Link))
	t.RawSetString("description", lua.LString(parsed.Description))
	items := ls.CreateTable(len(parsed.Items), 0)
	for i, it := range parsed.Items {
		items.RawSetInt(i+1, feedItemToLua(ls, it))
	}
	t.RawSetString("items", items)
	ls.Push(t)
	ls.Push(lua.LNil)
	return 2
}

func feedItemToLua(ls *lua.LState, it feed.Item) *lua.LTable {
	t := ls.NewTable()
	setOptional(t, "id", it.ID)
	t.RawSetString("title", lua.LString(it.Title))
	t.RawSetString("link", lua.LString(it.Link))
	t.RawSetString("summary", lua.LString(it.Summary))
	t.RawSetString("content", lua.LString(it.Content))
	setOptional(t, "published", formatFeedTime(it.Published))
	setOptional(t, "updated", formatFeedTime(it.Updated))
	setOptional(t, "published_raw", it.PublishedRaw)
	setOptional(t, "updated_raw", it.UpdatedRaw)
	t.RawSetString("authors", stringList(ls, it.Authors))
	t.RawSetString("categories", stringList(ls, it.Categories))
	return t
}

// setOptional sets key only for a non-empty value, so an absent value reads
// as nil in the script rather than as "".
func setOptional(t *lua.LTable, key, val string) {
	if val != "" {
		t.RawSetString(key, lua.LString(val))
	}
}

func formatFeedTime(tm time.Time) string {
	if tm.IsZero() {
		return ""
	}
	return tm.UTC().Format(time.RFC3339)
}

func stringList(ls *lua.LState, vals []string) *lua.LTable {
	t := ls.CreateTable(len(vals), 0)
	for i, v := range vals {
		t.RawSetInt(i+1, lua.LString(v))
	}
	return t
}
