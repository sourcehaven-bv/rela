package apptest

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	lua "github.com/yuin/gopher-lua"

	relalua "github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// The keys of a t.http.stub spec and of one response.
var (
	stubKeys = []string{"name", "method", "url", "url_prefix", "url_pattern", "headers", "body", "json", "form",
		"respond", "responses"}
	responseKeys = []string{"status", "headers", "body", "json", "file", "error"}
)

// httpModule holds the t.http functions of a file run.
type httpModule struct{ fr *fileRun }

// httpTable builds t.http.
func (h httpModule) httpTable(ls *lua.LState) *lua.LTable {
	fr := h.fr
	t := ls.NewTable()
	t.RawSetString("stub", ls.NewFunction(h.luaHTTPStub))
	t.RawSetString("calls", ls.NewFunction(func(ls *lua.LState) int {
		w := fr.world(ls, "http.calls")
		ls.Push(callsTable(ls, w.http.recorded(nil)))
		return 1
	}))
	return t
}

// luaHTTPStub is t.http.stub(spec): it registers a stub for the rest of the
// case and returns it, with :calls() and :count().
func (h httpModule) luaHTTPStub(ls *lua.LState) int {
	fr := h.fr
	w := fr.world(ls, "http.stub")
	spec := optionTable(ls, 1, stubKeys...)
	s := &httpStub{where: where(ls)}
	h.stubTarget(ls, spec, s)
	method, _ := optString(ls, spec, "method", "")
	s.method = strings.ToUpper(method)
	s.header = stringMap(ls, spec.RawGetString("headers"), "headers", true)
	if body, ok := optString(ls, spec, "body", ""); ok {
		s.body = &body
	}
	if j := spec.RawGetString("json"); j != lua.LNil {
		s.json = jsonValue(ls, j, "json")
	}
	if f := spec.RawGetString("form"); f != lua.LNil {
		s.form = stringMap(ls, f, "form", false)
	}
	h.stubResponses(ls, spec, s)
	s.name, _ = optString(ls, spec, "name", "")
	s.named = s.name != ""
	if !s.named {
		s.name = defaultStubName(s)
	}
	fr.check(ls, w.http.add(s))
	ls.Push(h.stubHandle(ls, w, s))
	return 1
}

// stubTarget reads the one URL condition of spec.
func (h httpModule) stubTarget(ls *lua.LState, spec *lua.LTable, s *httpStub) {
	exact, _ := optString(ls, spec, "url", "")
	prefix, _ := optString(ls, spec, "url_prefix", "")
	pattern, _ := optString(ls, spec, "url_pattern", "")
	set := 0
	for _, v := range []string{exact, prefix, pattern} {
		if v != "" {
			set++
		}
	}
	if set != 1 {
		ls.ArgError(1, "give exactly one of url, url_prefix and url_pattern")
	}
	switch {
	case exact != "":
		u, err := url.Parse(exact)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			ls.ArgError(1, fmt.Sprintf("url %q is not an absolute http or https URL", exact))
		}
		s.url = u
	case prefix != "":
		s.prefix = prefix
	default:
		re, err := regexp.Compile("^(?:" + pattern + ")$")
		if err != nil {
			ls.ArgError(1, fmt.Sprintf("url_pattern: %v", err))
		}
		s.pattern = re
	}
}

// stubResponses reads respond (one response, given for every call) or
// responses (a list, given once each in order).
func (h httpModule) stubResponses(ls *lua.LState, spec *lua.LTable, s *httpStub) {
	one, list := spec.RawGetString("respond"), spec.RawGetString("responses")
	switch {
	case one != lua.LNil && list != lua.LNil:
		ls.ArgError(1, "give respond or responses, not both")
	case list != lua.LNil:
		lt, ok := list.(*lua.LTable)
		if !ok || lt.Len() == 0 {
			ls.ArgError(1, "responses must be a non-empty list of responses")
		}
		for i := 1; i <= lt.Len(); i++ {
			s.responses = append(s.responses, h.stubResponse(ls, lt.RawGetInt(i), fmt.Sprintf("responses[%d]", i)))
		}
	case one != lua.LNil:
		s.responses, s.repeat = []stubResponse{h.stubResponse(ls, one, "respond")}, true
	default:
		// No answer given: 200 with an empty body, for every call.
		s.responses, s.repeat = []stubResponse{{status: http.StatusOK, header: http.Header{}}}, true
	}
}

// stubResponse reads one response table.
func (h httpModule) stubResponse(ls *lua.LState, v lua.LValue, name string) stubResponse {
	fr := h.fr
	t, ok := v.(*lua.LTable)
	if !ok {
		ls.ArgError(1, name+" must be a table")
	}
	checkKeys(ls, t, name, responseKeys)
	if kind, _ := optString(ls, t, "error", name+"."); kind != "" {
		if kind != stubErrTimeout && kind != stubErrNetwork {
			ls.ArgError(1, fmt.Sprintf("%s.error must be %q or %q", name, stubErrTimeout, stubErrNetwork))
		}
		t.ForEach(func(k, _ lua.LValue) {
			if k.String() != "error" {
				ls.ArgError(1, fmt.Sprintf("%s: an error response takes no %s", name, k.String()))
			}
		})
		return stubResponse{errKind: kind}
	}
	r := stubResponse{status: http.StatusOK, header: http.Header{}}
	if st := t.RawGetString("status"); st != lua.LNil {
		n, isNum := st.(lua.LNumber)
		if !isNum || float64(n) != float64(int(n)) || n < 100 || n > 599 {
			ls.ArgError(1, name+".status must be an HTTP status code from 100 to 599")
		}
		r.status = int(n)
	}
	for k, val := range stringMap(ls, t.RawGetString("headers"), name+".headers", true) {
		r.header.Set(k, val)
	}
	bodies := 0
	contentType := ""
	if b, isStr := optString(ls, t, "body", name+"."); isStr {
		r.body, bodies = []byte(b), bodies+1
	}
	if j := t.RawGetString("json"); j != lua.LNil {
		data, err := json.Marshal(jsonValue(ls, j, name+".json"))
		fr.check(ls, err)
		r.body, bodies, contentType = data, bodies+1, "application/json"
	}
	if f, isStr := optString(ls, t, "file", name+"."); isStr {
		r.body, bodies = h.fixtureFile(ls, f, name+".file"), bodies+1
		if strings.EqualFold(path.Ext(f), ".json") {
			contentType = "application/json"
		}
	}
	if bodies > 1 {
		ls.ArgError(1, name+": give at most one of body, json and file")
	}
	if contentType != "" && r.header.Get("Content-Type") == "" {
		r.header.Set("Content-Type", contentType)
	}
	return r
}

// optString reads the string at key of t, and whether it is set. Any other
// type is an argument error, so a mistyped key cannot widen a stub; prefix
// names the table in the message.
func optString(ls *lua.LState, t *lua.LTable, key, prefix string) (string, bool) {
	switch v := t.RawGetString(key).(type) {
	case *lua.LNilType:
		return "", false
	case lua.LString:
		return string(v), true
	default:
		ls.ArgError(1, fmt.Sprintf("%s%s must be a string, not a %s", prefix, key, v.Type()))
		return "", false
	}
}

// fixtureFile reads a response body file: a path relative to the project
// root, inside it, with no symlink on the way, as t.fixtures takes. The
// project's .rela directory is refused, because it holds secrets.yaml and a
// response body ends up in recorded calls and failure messages.
func (h httpModule) fixtureFile(ls *lua.LState, name, field string) []byte {
	fr := h.fr
	rel := filepath.FromSlash(name)
	if !filepath.IsLocal(rel) {
		ls.ArgError(1, fmt.Sprintf("%s: %s is outside the project", field, name))
	}
	// EqualFold: the project may be on a case-insensitive file system.
	if first, _, _ := strings.Cut(filepath.ToSlash(filepath.Clean(rel)), "/"); strings.EqualFold(first, project.CacheDir) {
		ls.ArgError(1, fmt.Sprintf("%s: %s is in %s, which may hold secrets", field, name, project.CacheDir))
	}
	if err := noSymlinks(fr.r.root, rel); err != nil {
		ls.ArgError(1, fmt.Sprintf("%s: %v", field, err))
	}
	data, err := fr.r.files.Load(fr.ls.Context(), filepath.ToSlash(rel))
	if err != nil {
		ls.ArgError(1, fmt.Sprintf("%s: %v", field, err))
	}
	return data
}

// defaultStubName names a stub after what it matches.
func defaultStubName(s *httpStub) string {
	method := s.method
	if method == "" {
		method = "*"
	}
	switch {
	case s.url != nil:
		return method + " " + s.url.String()
	case s.prefix != "":
		return method + " " + s.prefix + "*"
	default:
		return method + " ~" + strings.TrimSuffix(strings.TrimPrefix(s.pattern.String(), "^(?:"), ")$")
	}
}

// stubHandle is the Lua table t.http.stub returns.
func (h httpModule) stubHandle(ls *lua.LState, w *world, s *httpStub) *lua.LTable {
	fr := h.fr
	t := ls.NewTable()
	t.RawSetString("name", lua.LString(s.name))
	method := func(name string, fn func(ls *lua.LState, calls []recordedCall)) {
		t.RawSetString(name, ls.NewFunction(func(ls *lua.LState) int {
			if ls.Get(1) != t {
				ls.RaiseError("call stub methods with a colon: stub:%s()", name)
			}
			fr.world(ls, "stub:"+name)
			fn(ls, w.http.recorded(s))
			return 1
		}))
	}
	method("calls", func(ls *lua.LState, calls []recordedCall) { ls.Push(callsTable(ls, calls)) })
	method("count", func(ls *lua.LState, calls []recordedCall) { ls.Push(lua.LNumber(len(calls))) })
	return t
}

// callsTable renders recorded calls for Lua: method, url, headers (one
// value per canonical name, multiple values joined with ", "), body, json
// (the decoded body, or nil when it is not JSON) and stub (the name of the
// stub that answered; nil for an unmatched call).
func callsTable(ls *lua.LState, calls []recordedCall) *lua.LTable {
	out := ls.NewTable()
	for _, c := range calls {
		t := ls.NewTable()
		t.RawSetString("method", lua.LString(c.method))
		t.RawSetString("url", lua.LString(c.url))
		hdr := ls.NewTable()
		for _, k := range slices.Sorted(maps.Keys(c.header)) {
			hdr.RawSetString(k, lua.LString(strings.Join(c.header.Values(k), ", ")))
		}
		t.RawSetString("headers", hdr)
		t.RawSetString("body", lua.LString(c.body))
		if v := c.decodedJSON(); v != nil {
			t.RawSetString("json", relalua.GoToLuaValue(ls, v))
		}
		if c.stub != "" {
			t.RawSetString("stub", lua.LString(c.stub))
		}
		out.Append(t)
	}
	return out
}

// checkKeys raises an argument error for a key of t not in allowed.
func checkKeys(ls *lua.LState, t *lua.LTable, name string, allowed []string) {
	t.ForEach(func(k, _ lua.LValue) {
		key, ok := k.(lua.LString)
		if !ok || !slices.Contains(allowed, string(key)) {
			ls.ArgError(1, fmt.Sprintf("%s: unknown key %s; the keys are %s", name, k.String(),
				strings.Join(allowed, ", ")))
		}
	})
}

// stringMap reads a table of string keys to scalar values. With canonical
// set the keys are header names, put in canonical form.
func stringMap(ls *lua.LState, v lua.LValue, name string, canonical bool) map[string]string {
	if v == lua.LNil {
		return nil
	}
	t, ok := v.(*lua.LTable)
	if !ok {
		ls.ArgError(1, name+" must be a table of names to values")
	}
	out := map[string]string{}
	t.ForEach(func(k, val lua.LValue) {
		key, isStr := k.(lua.LString)
		if !isStr {
			ls.ArgError(1, name+" must be keyed by name")
		}
		switch val.(type) {
		case lua.LString, lua.LNumber, lua.LBool:
		default:
			ls.ArgError(1, fmt.Sprintf("%s.%s must be a string, number or boolean", name, string(key)))
		}
		k2 := string(key)
		if canonical {
			k2 = http.CanonicalHeaderKey(k2)
		}
		out[k2] = val.String()
	})
	return out
}

// jsonValue converts a Lua value to what encoding/json decodes the same
// JSON to, so numbers compare as float64.
func jsonValue(ls *lua.LState, v lua.LValue, name string) any {
	data, err := json.Marshal(relalua.ToGo(v))
	if err != nil {
		ls.ArgError(1, fmt.Sprintf("%s: %v", name, err))
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		ls.ArgError(1, fmt.Sprintf("%s: %v", name, err))
	}
	return out
}

// luaSecrets is t.secrets{name = value, ...}: fake secret values the case's
// scripts and webhooks see in place of .rela/secrets.yaml, which a case
// never reads. A script still sees only the names its capabilities list.
// Later calls add to and overwrite earlier ones.
func (fr *fileRun) luaSecrets(ls *lua.LState) int {
	w := fr.world(ls, "secrets")
	values := stringMap(ls, ls.CheckTable(1), "t.secrets", false)
	w.secrets.set(values)
	return 0
}
