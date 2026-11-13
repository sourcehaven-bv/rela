package apptest

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"

	lua "github.com/yuin/gopher-lua"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclmap"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	relalua "github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// luaExpect is t.expect(v): a table of matchers over v, called with a dot.
func (fr *fileRun) luaExpect(ls *lua.LState) int {
	fr.world(ls, "expect")
	got := ls.Get(1)
	m := ls.NewTable()
	matcher := func(name string, fn func(ls *lua.LState)) {
		m.RawSetString(name, ls.NewFunction(func(ls *lua.LState) int {
			if ls.Get(1) == m {
				ls.RaiseError("call matchers with a dot: t.expect(v).%s(...)", name)
			}
			fn(ls)
			return 0
		}))
	}
	matcher("to_be", func(ls *lua.LState) {
		want := ls.Get(1)
		if !ls.Equal(got, want) && !sameNumber(got, want) {
			fr.failNow(ls, "expected %s, got %s", describe(want), describe(got))
		}
	})
	matcher("to_equal", func(ls *lua.LState) {
		want := ls.Get(1)
		if !reflect.DeepEqual(normalize(relalua.ToGo(got)), normalize(relalua.ToGo(want))) {
			fr.failNow(ls, "expected %s, got %s", describe(want), describe(got))
		}
	})
	matcher("to_contain", func(ls *lua.LState) {
		want := ls.Get(1)
		if !contains(got, want) {
			fr.failNow(ls, "expected %s to contain %s", describe(got), describe(want))
		}
	})
	matcher("to_be_nil", func(ls *lua.LState) {
		if got != lua.LNil {
			fr.failNow(ls, "expected nil, got %s", describe(got))
		}
	})
	matcher("not_to_be_nil", func(ls *lua.LState) {
		if got == lua.LNil {
			fr.failNow(ls, "expected a value, got nil")
		}
	})
	ls.Push(m)
	return 1
}

// sameNumber reports whether a and b are numbers that compare equal.
func sameNumber(a, b lua.LValue) bool {
	x, ok1 := a.(lua.LNumber)
	y, ok2 := b.(lua.LNumber)
	return ok1 && ok2 && x == y
}

// contains reports whether a string holds a substring, or a list holds an
// element equal to want.
func contains(got, want lua.LValue) bool {
	switch g := got.(type) {
	case lua.LString:
		w, ok := want.(lua.LString)
		return ok && strings.Contains(string(g), string(w))
	case *lua.LTable:
		wv := normalize(relalua.ToGo(want))
		found := false
		g.ForEach(func(_, v lua.LValue) {
			if !found && reflect.DeepEqual(normalize(relalua.ToGo(v)), wv) {
				found = true
			}
		})
		return found
	}
	return false
}

// normalize makes values from Lua and from the store compare equal: an
// integral float becomes an int64, and lists and maps are normalized in
// depth.
func normalize(v any) any {
	switch x := v.(type) {
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1<<53 {
			return int64(x)
		}
		return x
	case int:
		return int64(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = normalize(e)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = normalize(e)
		}
		return out
	}
	return v
}

// describe renders a Lua value for a failure message.
func describe(v lua.LValue) string {
	switch x := v.(type) {
	case lua.LString:
		return fmt.Sprintf("%q", string(x))
	case *lua.LTable:
		return fmt.Sprintf("%v", normalize(relalua.ToGo(x)))
	}
	return v.String()
}

// call runs fn in protected mode and returns its error, if any.
func (fr *fileRun) call(ls *lua.LState, fn *lua.LFunction) error {
	if fr.current != nil {
		fr.current.lastErr = nil
	}
	ls.Push(fn)
	return ls.PCall(0, 0, nil)
}

// rethrowFailure ends the case when fn failed an assertion of its own:
// t.expect_error does not swallow a failed t.expect.
func (fr *fileRun) rethrowFailure(ls *lua.LState) {
	if fr.current != nil && fr.current.failure != nil {
		ls.RaiseError("%s", fr.current.failure.msg)
	}
}

// luaExpectError is t.expect_error(fn, pattern): fn must raise an error,
// and when pattern is given the message must contain it (plain text).
func (fr *fileRun) luaExpectError(ls *lua.LState) int {
	fr.world(ls, "expect_error")
	fn := ls.CheckFunction(1)
	pattern := ls.OptString(2, "")
	err := fr.call(ls, fn)
	fr.rethrowFailure(ls)
	if err == nil {
		fr.failNow(ls, "expected an error, got none")
	}
	msg := luaErrorText(err)
	if pattern != "" && !strings.Contains(msg, pattern) {
		fr.failNow(ls, "expected an error containing %q, got %q", pattern, msg)
	}
	ls.Push(lua.LString(msg))
	return 1
}

// luaExpectDenied is t.expect_denied(fn): fn must fail because the ACL
// refused a write: a forbidden error, or a not-found for an entity that
// exists but the actor may not read. Any other error fails the assertion,
// so a write that fails validation does not pass as denied. A read the ACL
// hides is not an error: actor:get returns nil, which t.expect(...).to_be_nil
// asserts.
func (fr *fileRun) luaExpectDenied(ls *lua.LState) int {
	w := fr.world(ls, "expect_denied")
	fn := ls.CheckFunction(1)
	err := fr.call(ls, fn)
	fr.rethrowFailure(ls)
	if err == nil {
		fr.failNow(ls, "expected the ACL to deny the call, but it succeeded")
	}
	// lastErr must be the error fn ended with, not one it caught earlier.
	var hidden hiddenTargetError
	denied := w.lastErr != nil && strings.HasSuffix(luaErrorText(err), w.lastErr.Error()) &&
		(errors.Is(w.lastErr, acl.ErrForbidden) || errors.As(w.lastErr, &hidden))
	if !denied {
		_, msg := splitLuaError(luaErrorText(err))
		fr.failNow(ls, "expected the ACL to deny the call, but it failed otherwise: %s", msg)
	}
	return 0
}

// mailTable converts a sent message for Lua.
func mailTable(ls *lua.LState, m mail.Message) *lua.LTable {
	t := ls.NewTable()
	to := ls.NewTable()
	for _, a := range m.To {
		to.Append(lua.LString(a.Email))
	}
	t.RawSetString("to", to)
	t.RawSetString("subject", lua.LString(m.Subject))
	t.RawSetString("text", lua.LString(string(m.Text)))
	t.RawSetString("html", lua.LString(string(m.HTML)))
	return t
}

// luaExpectMail is t.expect_mail{to=, subject=, contains=}: some message
// sent in the case must go to `to`, have a subject containing `subject`, and
// a text or HTML body containing `contains`. Every field is optional. It
// returns the first matching message.
func (fr *fileRun) luaExpectMail(ls *lua.LState) int {
	w := fr.world(ls, "expect_mail")
	spec := optionTable(ls, 1, "to", "subject", "contains")
	to := lua.LVAsString(spec.RawGetString("to"))
	subject := lua.LVAsString(spec.RawGetString("subject"))
	body := lua.LVAsString(spec.RawGetString("contains"))
	sent := w.mail.Messages()
	for _, m := range sent {
		if to != "" && !slices.ContainsFunc(m.To, func(a mail.Address) bool { return strings.EqualFold(a.Email, to) }) {
			continue
		}
		if !strings.Contains(m.Subject, subject) {
			continue
		}
		if body != "" && !strings.Contains(string(m.Text), body) && !strings.Contains(string(m.HTML), body) {
			continue
		}
		ls.Push(mailTable(ls, m))
		return 1
	}
	var seen []string
	for _, m := range sent {
		var rcpt []string
		for _, a := range m.To {
			rcpt = append(rcpt, a.Email)
		}
		seen = append(seen, fmt.Sprintf("%q to %s", m.Subject, strings.Join(rcpt, ",")))
	}
	if len(seen) == 0 {
		fr.failNow(ls, "expected a mail, but none was sent")
	}
	fr.failNow(ls, "expected a mail matching %s; sent: %s", describe(spec), strings.Join(seen, "; "))
	return 0
}

// luaExpectAccess is t.expect_access(user, expectation): user's access must
// be exactly what the table lists, in the format of `rela acl map --expect`
// (types, permissions, relations).
func (fr *fileRun) luaExpectAccess(ls *lua.LState) int {
	w := fr.world(ls, "expect_access")
	user := ls.CheckString(1)
	spec := ls.CheckTable(2)
	fr.check(ls, w.ensure())
	decl, pol := w.svc.ACLDeclarative(), w.svc.ACLPolicy()
	if decl == nil || pol == nil {
		fr.raise(ls, errors.New("t.expect_access needs acl.yaml: without a policy every principal has full access"))
	}
	data, err := yaml.Marshal(relalua.ToGo(spec))
	fr.check(ls, err)
	loc := where(ls)
	x, err := aclmap.ParseExpectation(data, loc, w.svc.Meta())
	fr.check(ls, err)
	meta := w.svc.Meta()
	engine, err := aclmap.New(w.svc.Store(), decl, func(typ string) []entity.Face {
		names := metamodel.FaceOrderOf(meta, typ)
		faces := make([]entity.Face, len(names))
		for i, n := range names {
			faces[i] = entity.Face(n)
		}
		return faces
	})
	fr.check(ls, err)
	res, err := engine.MapPrincipalFull(w.systemContext(), user, "", "", meta.EntityTypes(),
		aclmap.ClientView{}, aclmap.ExtrasFor(meta, pol))
	fr.check(ls, err)
	cmp := x.Compare(loc, res)
	if !cmp.Match {
		fr.failNow(ls, "access of %s differs: unexpected %s; missing %s",
			user, capabilities(cmp.Unexpected), capabilities(cmp.Missing))
	}
	return 0
}

// capabilities renders a capability list for a failure message.
func capabilities(cs []aclmap.Capability) string {
	if len(cs) == 0 {
		return "none"
	}
	out := make([]string, len(cs))
	for i, c := range cs {
		s := c.Section + " "
		if c.Key != "" {
			s += c.Key + " "
		}
		out[i] = s + c.Value
	}
	return strings.Join(out, ", ")
}
