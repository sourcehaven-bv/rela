package lua

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	glua "github.com/yuin/gopher-lua"
)

// luaURL implements rela.url(path, params?).
//
//   - path: literal path (e.g. "/form/full_ticket/TKT-001"), may carry an
//     existing query and/or fragment.
//   - params: optional table of extra query values. Map keys are strings;
//     values may be strings, numbers, or booleans. Non-scalar values raise.
//
// The base path (stripped of query + fragment) is verified against the
// route catalog. Unknown paths raise "unknown frontend route: <path>".
// The returned string carries the original base path, a merged query with
// deterministic key ordering, and the original fragment.
func (r *Runtime) luaURL(ls *glua.LState) int {
	// r.routes is guaranteed non-nil here: runtime.go registers this
	// binding only when a catalog was wired (see registerContextBindings).
	rawPath := ls.CheckString(1)
	base, existingQuery, fragment := splitPathQueryFragment(rawPath)

	if !r.routes.Has(base) {
		ls.RaiseError("unknown frontend route: %s", base)
		return 0
	}

	values, err := existingQueryValues(existingQuery)
	if err != nil {
		ls.RaiseError("rela.url: invalid query on path %q: %s", rawPath, err.Error())
		return 0
	}

	// Merge the optional params table.
	if ls.GetTop() >= 2 && ls.Get(2) != glua.LNil {
		t := ls.CheckTable(2)
		mergeErr := mergeParamsTable(t, values)
		if mergeErr != nil {
			ls.RaiseError("rela.url: %s", mergeErr.Error())
			return 0
		}
	}

	ls.Push(glua.LString(buildURL(base, values, fragment)))
	return 1
}

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
