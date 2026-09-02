package lua

import (
	lua "github.com/yuin/gopher-lua"
)

// Request is the inbound HTTP request a request-scoped action script sees as
// `rela.request` (TKT-EFMRQM).
//
// It is a VALUE, deliberately not an *http.Request: the lua package must not
// be handed a live request it could read a cookie or a bearer token off. The
// caller projects exactly what the action's `request:` config opted into, and
// nothing else can be reached from Lua however the script is written.
//
// Nil: accepted at [WithRequest] — a nil Request leaves `rela.request` absent,
// which is what an action with no `request:` block gets.
type Request struct {
	// Method and Path describe the call. Both are server-derived, not
	// caller-asserted, so they are always safe to expose.
	Method string
	Path   string

	// ContentType is the raw Content-Type header value. Exposed unconditionally
	// with the body because a script that reads a body needs to know how to
	// interpret it, and it carries no credential.
	ContentType string

	// Raw is the request body verbatim, already bounded by the caller's size
	// cap. Empty when the action did not opt into the body.
	Raw string

	// Body is the parsed body — a JSON object, or a flat map for a form
	// encoding. Nil when the body was not requested, was empty, or did not
	// parse; a script distinguishes those via Raw.
	Body map[string]any

	// Query is the URL query, one Lua string per key (the FIRST value, matching
	// url.Values.Get) plus a `_all` list. Nil when not requested.
	Query map[string][]string

	// Headers holds ONLY the headers the action's config allowlisted, keyed
	// lowercase.
	Headers map[string]string
}

// WithRequest exposes an inbound HTTP request to the script as `rela.request`.
// Passing nil (the default) leaves the field absent.
func WithRequest(req *Request) Option {
	return func(r *Runtime) {
		r.request = req
	}
}

// registerRequestBinding installs the frozen `rela.request` table. Absent
// entirely when no request was supplied, so `if rela.request then` is the
// script-side test for request-scoped execution.
//
// Frozen with [freezeTable] like rela.principal: the table is the record of
// what actually arrived on the wire, and a script that rewrote it would make a
// later read of the same field — an error envelope, a log line — disagree with
// what the producer sent.
func (r *Runtime) registerRequestBinding(rela *lua.LTable) {
	if r.request == nil {
		return
	}
	req := r.request
	ls := r.L

	data := ls.NewTable()
	ls.SetField(data, "method", lua.LString(req.Method))
	ls.SetField(data, "path", lua.LString(req.Path))
	ls.SetField(data, "content_type", lua.LString(req.ContentType))
	ls.SetField(data, "raw", lua.LString(req.Raw))

	if req.Body != nil {
		// Reuse the JSON converter so a request body and a json.decode() result
		// have identical Lua shapes, including the depth cap that keeps a
		// deeply-nested payload from overflowing the Lua stack.
		ls.SetField(data, "body", goJSONToLua(ls, req.Body))
	}

	if req.Query != nil {
		query := ls.NewTable()
		all := ls.NewTable()
		for key, values := range req.Query {
			if len(values) == 0 {
				continue
			}
			// The scalar is the FIRST value, matching url.Values.Get and every
			// other query read in this codebase. A repeated parameter is
			// reachable in full through `_all`, so nothing is silently lost —
			// but the common `rela.request.query.id` stays a string rather
			// than sometimes being a table, which is the shape that produces
			// a script that works until the day a caller repeats a key.
			query.RawSetString(key, lua.LString(values[0]))
			list := ls.NewTable()
			for i, v := range values {
				list.RawSetInt(i+1, lua.LString(v))
			}
			all.RawSetString(key, list)
		}
		query.RawSetString("_all", all)
		ls.SetField(data, "query", freezeTable(ls, query))
	}

	headers := ls.NewTable()
	for k, v := range req.Headers {
		headers.RawSetString(k, lua.LString(v))
	}
	ls.SetField(data, "headers", freezeTable(ls, headers))

	ls.SetField(rela, "request", freezeTable(ls, data))
}
