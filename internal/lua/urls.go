package lua

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	glua "github.com/yuin/gopher-lua"
)

// registerURLModule installs the rela.url submodule. rela.url is itself
// callable (rela.url("/path", params?)) via a __call metamethod, and also
// exposes typed helpers for each frontend-route kind: form, detail, list,
// view, kanban, document. Every helper returns a string URL, verified
// against the route catalog (so a typo like rela.url.form("nope", e) fails
// loudly at render time instead of a 404 in the browser).
//
// The registration is no-op when no catalog is wired — same pattern as
// rela.cache — so runtimes outside document renders don't expose rela.url
// at all.
func (r *Runtime) registerURLModule(rela *glua.LTable) {
	if r.routes == nil {
		return
	}

	tbl := r.L.NewTable()
	r.L.SetField(tbl, "form", r.L.NewFunction(r.luaURLForm))
	r.L.SetField(tbl, "detail", r.L.NewFunction(r.luaURLDetail))
	r.L.SetField(tbl, "list", r.L.NewFunction(r.luaURLList))
	r.L.SetField(tbl, "view", r.L.NewFunction(r.luaURLView))
	r.L.SetField(tbl, "kanban", r.L.NewFunction(r.luaURLKanban))
	r.L.SetField(tbl, "document", r.L.NewFunction(r.luaURLDocument))

	// Make the table callable so rela.url("/any/path", params?) still
	// works as an escape hatch for routes without a dedicated helper
	// (e.g. /search) or for paths the author has assembled themselves.
	mt := r.L.NewTable()
	r.L.SetField(mt, "__call", r.L.NewFunction(r.luaURLCall))
	r.L.SetMetatable(tbl, mt)

	r.L.SetField(rela, "url", tbl)
}

// luaURLCall handles rela.url(path, params?) — the __call path. The first
// Lua argument is the rela.url table itself (self); the real args start at
// index 2. We remove the self arg and delegate to the primitive.
func (r *Runtime) luaURLCall(ls *glua.LState) int {
	ls.Remove(1)
	return r.luaURLPath(ls)
}

// luaURLPath is the catalog-verified primitive: path string + optional
// params table → URL string. Factored out from the callable wrapper so
// helpers below can share the query-merge + verification logic.
//
//   - path: literal path (e.g. "/form/full_ticket/TKT-001"), may carry an
//     existing query and/or fragment.
//   - params: optional table of extra query values. Map keys are strings;
//     values may be strings, numbers, or booleans. Non-scalar values raise.
//
// Unknown paths raise "unknown frontend route: <path>". The returned string
// carries the original base path, a merged query with deterministic key
// ordering, and the original fragment.
func (r *Runtime) luaURLPath(ls *glua.LState) int {
	rawPath := ls.CheckString(1)
	out, err := r.buildVerifiedURL(rawPath, optionalTable(ls, 2))
	if err != nil {
		ls.RaiseError("%s", err.Error())
		return 0
	}
	ls.Push(glua.LString(out))
	return 1
}

// buildVerifiedURL is the engine behind every url helper. It splits the
// raw path, verifies it against the catalog, merges any existing query
// with the caller-supplied params map, and returns the final string.
// All Go-side helpers (luaURLForm, luaURLDetail, ...) assemble a raw path
// and delegate here.
func (r *Runtime) buildVerifiedURL(rawPath string, extra *glua.LTable) (string, error) {
	base, existingQuery, fragment := splitPathQueryFragment(rawPath)
	if !r.routes.Has(base) {
		return "", fmt.Errorf("unknown frontend route: %s", base)
	}
	values, err := existingQueryValues(existingQuery)
	if err != nil {
		return "", fmt.Errorf("rela.url: invalid query on path %q: %s", rawPath, err.Error())
	}
	if extra != nil {
		if mergeErr := mergeParamsTable(extra, values); mergeErr != nil {
			return "", fmt.Errorf("rela.url: %s", mergeErr.Error())
		}
	}
	return buildURL(base, values, fragment), nil
}

// optionalTable reads argument at position n, returning nil if absent or
// glua.LNil. Raises a typed Lua error if present but not a table.
func optionalTable(ls *glua.LState, n int) *glua.LTable {
	if ls.GetTop() < n {
		return nil
	}
	v := ls.Get(n)
	if v == glua.LNil {
		return nil
	}
	return ls.CheckTable(n)
}

// -----------------------------------------------------------------------------
// Typed helpers (rela.url.form, rela.url.detail, ...)
// -----------------------------------------------------------------------------

// luaURLForm implements rela.url.form(name, arg?).
//
// When arg is an entity table (has a string `id` field), builds
// /form/<name>/<id> — edit mode.
//
// Otherwise arg is treated as an opts table with optional keys:
//
//	relations  = {name = target_id, ...}   → rel.<name>=<target_id>
//	properties = {name = value, ...}       → prop.<name>=<value>
//	query      = {k = v, ...}              → k=v (verbatim)
//
// and builds /form/<name>?<query> — create mode.
func (r *Runtime) luaURLForm(ls *glua.LState) int {
	name := ls.CheckString(1)
	if name == "" {
		ls.RaiseError("rela.url.form: form name cannot be empty")
		return 0
	}

	if ls.GetTop() < 2 || ls.Get(2) == glua.LNil {
		// rela.url.form("foo") — bare create form, no pre-fill.
		return r.emitURL(ls, "/form/"+name, nil)
	}

	arg := ls.CheckTable(2)
	if entityID := entityIDOf(arg); entityID != "" {
		// Edit mode: /form/<name>/<id>. Pass any extra query the caller
		// squeezed in via a "query" key — rare, but honest.
		if extra := tableField(arg, "query"); extra != nil {
			return r.emitURL(ls, "/form/"+name+"/"+entityID, extra)
		}
		return r.emitURL(ls, "/form/"+name+"/"+entityID, nil)
	}

	// Create mode: fold relations / properties / query into one query map.
	query, err := foldFormOpts(arg)
	if err != nil {
		ls.RaiseError("rela.url.form: %s", err.Error())
		return 0
	}
	return r.emitURLFromMap(ls, "/form/"+name, query)
}

// luaURLDetail implements rela.url.detail(entity).
// Returns /entity/<entity.type>/<entity.id> — the canonical detail page.
// No form choice, so no ambiguity.
func (r *Runtime) luaURLDetail(ls *glua.LState) int {
	entity := ls.CheckTable(1)
	id := entityIDOf(entity)
	typ := entityTypeOf(entity)
	if id == "" || typ == "" {
		ls.RaiseError(`rela.url.detail: entity must be a table with string "id" and "type" fields`)
		return 0
	}
	return r.emitURL(ls, "/entity/"+typ+"/"+id, nil)
}

// luaURLList implements rela.url.list(name, opts?).
// opts.query is optional extra query params.
func (r *Runtime) luaURLList(ls *glua.LState) int {
	name := ls.CheckString(1)
	if name == "" {
		ls.RaiseError("rela.url.list: list name cannot be empty")
		return 0
	}
	extra := optsQuery(ls, 2)
	return r.emitURL(ls, "/list/"+name, extra)
}

// luaURLView implements rela.url.view(name, entity).
func (r *Runtime) luaURLView(ls *glua.LState) int {
	name := ls.CheckString(1)
	if name == "" {
		ls.RaiseError("rela.url.view: view name cannot be empty")
		return 0
	}
	entity := ls.CheckTable(2)
	id := entityIDOf(entity)
	if id == "" {
		ls.RaiseError(`rela.url.view: entity must be a table with a string "id" field`)
		return 0
	}
	return r.emitURL(ls, "/view/"+name+"/"+id, nil)
}

// luaURLKanban implements rela.url.kanban(name, opts?).
func (r *Runtime) luaURLKanban(ls *glua.LState) int {
	name := ls.CheckString(1)
	if name == "" {
		ls.RaiseError("rela.url.kanban: kanban name cannot be empty")
		return 0
	}
	extra := optsQuery(ls, 2)
	return r.emitURL(ls, "/kanban/"+name, extra)
}

// luaURLDocument implements rela.url.document(name, entity).
func (r *Runtime) luaURLDocument(ls *glua.LState) int {
	name := ls.CheckString(1)
	if name == "" {
		ls.RaiseError("rela.url.document: document name cannot be empty")
		return 0
	}
	entity := ls.CheckTable(2)
	id := entityIDOf(entity)
	if id == "" {
		ls.RaiseError(`rela.url.document: entity must be a table with a string "id" field`)
		return 0
	}
	return r.emitURL(ls, "/document/"+name+"/"+id, nil)
}

// -----------------------------------------------------------------------------
// Small helpers shared across the typed bindings.
// -----------------------------------------------------------------------------

// emitURL verifies a helper-built path + optional Lua extra-query table
// and pushes the resulting URL onto the Lua stack. Raises on verification
// failure.
func (r *Runtime) emitURL(ls *glua.LState, path string, extra *glua.LTable) int {
	out, err := r.buildVerifiedURL(path, extra)
	if err != nil {
		ls.RaiseError("%s", err.Error())
		return 0
	}
	ls.Push(glua.LString(out))
	return 1
}

// emitURLFromMap is the same as emitURL but takes a pre-folded Go map —
// used when a helper has already walked Lua tables (e.g. form's create mode).
func (r *Runtime) emitURLFromMap(ls *glua.LState, path string, query map[string]string) int {
	base, existingQuery, fragment := splitPathQueryFragment(path)
	if !r.routes.Has(base) {
		ls.RaiseError("unknown frontend route: %s", base)
		return 0
	}
	values, err := existingQueryValues(existingQuery)
	if err != nil {
		ls.RaiseError("rela.url: invalid query on path %q: %s", path, err.Error())
		return 0
	}
	for k, v := range query {
		values[k] = v
	}
	ls.Push(glua.LString(buildURL(base, values, fragment)))
	return 1
}

// entityIDOf returns the string id from an entity-shaped Lua table, or ""
// if it's missing or the wrong type. Accepts the shape rela.get_entity
// returns (a table with at least id + type + properties).
func entityIDOf(t *glua.LTable) string {
	v := t.RawGetString("id")
	if s, ok := v.(glua.LString); ok {
		return string(s)
	}
	return ""
}

func entityTypeOf(t *glua.LTable) string {
	v := t.RawGetString("type")
	if s, ok := v.(glua.LString); ok {
		return string(s)
	}
	return ""
}

// tableField returns the *glua.LTable at key, or nil if absent or wrong type.
func tableField(t *glua.LTable, key string) *glua.LTable {
	v := t.RawGetString(key)
	if tbl, ok := v.(*glua.LTable); ok {
		return tbl
	}
	return nil
}

// optsQuery reads a {query = {...}} field from an options-table argument.
// Returns nil when the arg is absent, nil, or has no query field.
func optsQuery(ls *glua.LState, n int) *glua.LTable {
	opts := optionalTable(ls, n)
	if opts == nil {
		return nil
	}
	return tableField(opts, "query")
}

// foldFormOpts builds a Go query map from a form-opts table that may
// contain `relations`, `properties`, and/or `query` sub-tables. The
// `rel.` / `prop.` prefixes are added here so callers write bare
// relation and property names.
func foldFormOpts(opts *glua.LTable) (map[string]string, error) {
	out := map[string]string{}
	if rels := tableField(opts, "relations"); rels != nil {
		if err := foldPrefixed(rels, "rel.", out); err != nil {
			return nil, fmt.Errorf("relations: %s", err.Error())
		}
	}
	if props := tableField(opts, "properties"); props != nil {
		if err := foldPrefixed(props, "prop.", out); err != nil {
			return nil, fmt.Errorf("properties: %s", err.Error())
		}
	}
	if query := tableField(opts, "query"); query != nil {
		if err := mergeParamsTable(query, out); err != nil {
			return nil, fmt.Errorf("query: %s", err.Error())
		}
	}
	return out, nil
}

// foldPrefixed walks a Lua table of {name = value} and writes
// prefix+name = stringified-value into out. Reuses the scalar-validation
// rules from mergeParamsTable's helpers.
func foldPrefixed(t *glua.LTable, prefix string, out map[string]string) error {
	var err error
	t.ForEach(func(k, v glua.LValue) {
		if err != nil {
			return
		}
		key, ok := k.(glua.LString)
		if !ok {
			err = fmt.Errorf("keys must be strings, got %s", k.Type().String())
			return
		}
		keyStr := string(key)
		if keyStr == "" {
			err = errors.New("key cannot be empty")
			return
		}
		if strings.ContainsAny(keyStr, "&= \t\n\r.") {
			err = fmt.Errorf("key %q contains forbidden characters", keyStr)
			return
		}
		val, cErr := scalarToString(v)
		if cErr != nil {
			err = fmt.Errorf("value for key %q: %s", keyStr, cErr.Error())
			return
		}
		out[prefix+keyStr] = val
	})
	return err
}

// -----------------------------------------------------------------------------
// Primitive query-building shared with the __call path.
// -----------------------------------------------------------------------------

// splitPathQueryFragment returns (path, rawQuery, fragment). Each returned
// string is the raw character sequence — fragment/query markers stripped.
// Empty values are returned as empty strings.
func splitPathQueryFragment(raw string) (path, rawQuery, fragment string) {
	path = raw
	if i := strings.Index(path, "#"); i >= 0 {
		fragment = path[i+1:]
		path = path[:i]
	}
	if i := strings.Index(path, "?"); i >= 0 {
		rawQuery = path[i+1:]
		path = path[:i]
	}
	return path, rawQuery, fragment
}

// existingQueryValues parses an existing raw query string into an ordered
// map. We don't use url.ParseQuery directly because we want to preserve
// insertion semantics and reject genuinely malformed input loudly.
func existingQueryValues(rawQuery string) (map[string]string, error) {
	out := map[string]string{}
	if rawQuery == "" {
		return out, nil
	}
	for _, pair := range strings.Split(rawQuery, "&") {
		if pair == "" {
			continue
		}
		k, v, _ := strings.Cut(pair, "=")
		decodedKey, err := url.QueryUnescape(k)
		if err != nil {
			return nil, fmt.Errorf("bad query key %q: %w", k, err)
		}
		decodedVal, err := url.QueryUnescape(v)
		if err != nil {
			return nil, fmt.Errorf("bad query value for key %q: %w", decodedKey, err)
		}
		out[decodedKey] = decodedVal
	}
	return out, nil
}

// mergeParamsTable copies keys from a Lua table into the query map. Later
// writes win (params override any existing-query values with the same key).
// Keys must be non-empty strings without &, =, or whitespace. Values must
// be string, number, or bool. The key "return_to" is reserved (the document
// link rewriter injects it) and is rejected here so authors can't silently
// collide with it.
func mergeParamsTable(t *glua.LTable, out map[string]string) error {
	var err error
	t.ForEach(func(k, v glua.LValue) {
		if err != nil {
			return
		}
		key, ok := k.(glua.LString)
		if !ok {
			err = fmt.Errorf("param keys must be strings, got %s", k.Type().String())
			return
		}
		keyStr := string(key)
		if keyStr == "" {
			err = errors.New("param key cannot be empty")
			return
		}
		if keyStr == "return_to" {
			err = errors.New(`param key "return_to" is reserved; set by the document link rewriter`)
			return
		}
		if strings.ContainsAny(keyStr, "&= \t\n\r") {
			err = fmt.Errorf("param key %q contains forbidden characters (& = or whitespace)", keyStr)
			return
		}
		val, cErr := scalarToString(v)
		if cErr != nil {
			err = fmt.Errorf("param %q: %s", keyStr, cErr.Error())
			return
		}
		out[keyStr] = val
	})
	return err
}

func scalarToString(v glua.LValue) (string, error) {
	switch val := v.(type) {
	case glua.LString:
		return string(val), nil
	case glua.LNumber:
		// Avoid scientific notation for whole numbers; %v matches gopher-lua's
		// own string coercion.
		return val.String(), nil
	case glua.LBool:
		if bool(val) {
			return "true", nil
		}
		return "false", nil
	case *glua.LNilType:
		return "", errors.New("value is nil")
	default:
		return "", fmt.Errorf("value must be string, number, or boolean, got %s", v.Type().String())
	}
}

// buildURL reassembles base path, query (sorted for determinism), fragment.
// Empty query produces no "?"; empty fragment produces no "#".
func buildURL(base string, query map[string]string, fragment string) string {
	var b strings.Builder
	b.WriteString(base)
	if len(query) > 0 {
		keys := make([]string, 0, len(query))
		for k := range query {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('?')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte('&')
			}
			// mergeParamsTable rejects keys containing &, =, or whitespace, and
			// url.QueryEscape does not touch '.', so dotted keys like
			// "prop.status" round-trip unchanged.
			b.WriteString(url.QueryEscape(k))
			b.WriteByte('=')
			b.WriteString(url.QueryEscape(query[k]))
		}
	}
	if fragment != "" {
		b.WriteByte('#')
		b.WriteString(fragment)
	}
	return b.String()
}
