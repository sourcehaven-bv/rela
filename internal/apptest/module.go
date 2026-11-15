package apptest

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	relalua "github.com/Sourcehaven-BV/rela/internal/lua"
)

// The years t.clock accepts: a clock is kept in Unix nanoseconds, which
// int64 holds from 1677 to 2262.
const (
	minClockYear = 1678
	maxClockYear = 2261
)

// day is the unit of t.days.
const day = 24 * time.Hour

// module builds the table `require("rela.test")` returns.
//
// Cases and setup:
//
//	t.case(name, fn)           declare a case; only at the top level of a file
//	t.fixtures(dir, ...)       load the entity and relation markdown under each
//	                           directory (relative to the project root) as given
//	                           state, before the case's first write or read
//	t.principal{name=, roles=} declare a principal holding roles from acl.yaml;
//	                           returns an actor. Before the first write or read
//	t.as(user)                 an actor for user, with the roles the real
//	                           acl.yaml gives it
//
// Actor methods (colon syntax): create(type, props, opts), update(ref, props,
// opts), transition(ref, value, prop), delete(ref, opts), link(from, type, to,
// props), unlink(from, type, to), get(ref), list(type), start_flow(name, ref).
// A ref is an id, an "ID@face" address or an entity table. A failed write
// raises an error.
//
// HTTP: every outbound call of the case's code (Lua http.* and flow
// webhooks) is answered by a stub or fails the case. Nothing reaches the
// network.
//
//	t.http.stub{...}           register a stub for the rest of the case and
//	                           return it. The first registered stub that
//	                           matches and is not spent answers. Keys:
//	  name                     shown in failures and on recorded calls
//	  method                   any method when absent
//	  url | url_prefix |       exactly one: the full URL (query order, host
//	  url_pattern              case and a default port do not matter), a
//	                           plain string prefix (end it with "/", or
//	                           "https://api.example.com" also matches
//	                           "https://api.example.com.evil/"), or an RE2
//	                           expression that must match the whole URL
//	  headers                  {name = exact value}; names are case-insensitive;
//	                           the first value of a repeated header counts,
//	                           and Host and Content-Length cannot be matched
//	  body | json | form       the exact body; a JSON subset (objects by key,
//	                           arrays element by element at the same length;
//	                           an empty table matches any object or array);
//	                           form fields, urlencoded or multipart, where
//	                           the first value of a repeated field counts
//	  respond                  one response, given for every call
//	  responses                a list, given once each in order; a further
//	                           call that matches fails the case
//	A response is {status=, headers=, body= | json= | file=} or
//	{error = "timeout" | "network"}. file is a path under the project root,
//	with no symlink, as t.fixtures takes, and not under .rela. json and a .json file set
//	Content-Type unless headers do. No respond gives 200 with an empty body.
//	stub.name                  the given or generated name
//	stub:calls(), stub:count() the calls the stub answered, and how many
//	t.http.calls()             every call in order, matched or not: method,
//	                           url, headers (canonical names), body, json
//	                           (the decoded body, when it is JSON) and stub
//
// Secrets: t.secrets{name = value} gives the case's scripts and webhook
// signatures fake secrets. A case never reads .rela/secrets.yaml.
//
// Flows: job attempts wait until t.flows.work(), which runs them and the
// engine's due rounds at the case's clock and returns how many attempts ran.
// A retry is due after its backoff, so move the clock with t.advance first.
// t.flows.run(id) returns {id, flow, state, status, job}, where job has
// phase, attempts, failed, fail_kind and next_at while the step's job is
// pending.
//
// Assertions: t.expect(v).to_be(x) / to_equal(x) / to_contain(x) / to_be_nil()
// / not_to_be_nil(); t.expect_error(fn, pattern); t.expect_denied(fn);
// t.expect_mail{to=, subject=, contains=}; t.expect_access(user, table);
// t.fail(msg). The first failed assertion ends the case and names its line.
//
// Time and data: t.clock(iso), t.advance(seconds), t.now(), t.today(),
// t.days(n), t.hours(n), t.minutes(n), t.seconds(n); t.random(...);
// t.fake.name(), email(), sentence(), word(), date{within=days}, pick(list),
// value(type, prop), entity(type); t.mail().
func (fr *fileRun) module(ls *lua.LState) lua.LValue {
	t := ls.NewTable()
	fns := map[string]lua.LGFunction{
		"case":          fr.luaCase,
		"fixtures":      fr.luaFixtures,
		"principal":     fr.luaPrincipal,
		"as":            fr.luaAs,
		"expect":        fr.luaExpect,
		"expect_error":  fr.luaExpectError,
		"expect_denied": fr.luaExpectDenied,
		"expect_mail":   fr.luaExpectMail,
		"expect_access": fr.luaExpectAccess,
		"fail":          fr.luaFail,
		"mail":          fr.luaMail,
		"clock":         fr.luaClock,
		"advance":       fr.luaAdvance,
		"now":           fr.luaNow,
		"today":         fr.luaToday,
		"random":        fr.luaRandom,
		"days":          durationFn(day),
		"hours":         durationFn(time.Hour),
		"minutes":       durationFn(time.Minute),
		"seconds":       durationFn(time.Second),
		"secrets":       fr.luaSecrets,
	}
	for name, fn := range fns {
		t.RawSetString(name, ls.NewFunction(fn))
	}
	t.RawSetString("fake", fr.fakeTable(ls))
	t.RawSetString("http", httpModule{fr}.httpTable(ls))
	t.RawSetString("flows", flowsModule{fr}.flowsTable(ls))
	return t
}

// world returns the running case's world, or raises an error naming fn when
// it is called outside a case.
func (fr *fileRun) world(ls *lua.LState, fn string) *world {
	if fr.current == nil {
		ls.RaiseError("t.%s can only be called inside a case", fn)
	}
	return fr.current
}

// optionTable reads the optional table argument n and raises an argument
// error for a key not in allowed, so a misspelled option fails the case
// instead of being ignored. It returns an empty table when n is absent.
func optionTable(ls *lua.LState, n int, allowed ...string) *lua.LTable {
	t := ls.OptTable(n, nil)
	if t == nil {
		return ls.NewTable()
	}
	t.ForEach(func(k, _ lua.LValue) {
		name, ok := k.(lua.LString)
		if !ok || !slices.Contains(allowed, string(name)) {
			ls.ArgError(n, fmt.Sprintf("unknown option %s; the options are %s", k.String(), strings.Join(allowed, ", ")))
		}
	})
	return t
}

// raise ends the running Lua code with err, and records it for
// t.expect_denied and t.expect_error.
func (fr *fileRun) raise(ls *lua.LState, err error) {
	if fr.current != nil {
		fr.current.lastErr = err
	}
	ls.RaiseError("%s", err.Error())
}

// check raises err when it is not nil.
func (fr *fileRun) check(ls *lua.LState, err error) {
	if err != nil {
		fr.raise(ls, err)
	}
}

// failNow records a failed assertion at the calling line and ends the case.
func (fr *fileRun) failNow(ls *lua.LState, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fr.current != nil {
		fr.current.fail(where(ls), msg)
	}
	ls.RaiseError("%s", msg)
}

func (fr *fileRun) luaCase(ls *lua.LState) int {
	name := ls.CheckString(1)
	fn := ls.CheckFunction(2)
	if fr.current != nil {
		ls.RaiseError("t.case cannot be called inside a case")
	}
	if strings.TrimSpace(name) == "" {
		ls.ArgError(1, "a case needs a name")
	}
	for _, c := range fr.cases {
		if c.name == name {
			ls.RaiseError("case %q is already declared at %s", name, c.where)
		}
	}
	fr.cases = append(fr.cases, testCase{name: name, fn: fn, where: where(ls)})
	return 0
}

func (fr *fileRun) luaFixtures(ls *lua.LState) int {
	w := fr.world(ls, "fixtures")
	if w.built() {
		ls.RaiseError("t.fixtures must come before the case's first write or read")
	}
	if ls.GetTop() == 0 {
		ls.ArgError(1, "name at least one fixture directory")
	}
	for i := 1; i <= ls.GetTop(); i++ {
		w.fixtures = append(w.fixtures, ls.CheckString(i))
	}
	return 0
}

func (fr *fileRun) luaPrincipal(ls *lua.LState) int {
	w := fr.world(ls, "principal")
	spec := optionTable(ls, 1, "name", "roles")
	name := lua.LVAsString(spec.RawGetString("name"))
	if name == "" {
		name = fmt.Sprintf("principal-%d", len(w.principals)+1)
	}
	var roles []string
	switch rv := spec.RawGetString("roles").(type) {
	case *lua.LNilType:
	case *lua.LTable:
		rv.ForEach(func(_, v lua.LValue) {
			roles = append(roles, lua.LVAsString(v))
		})
	default:
		ls.ArgError(1, "roles must be a list of role names")
	}
	if _, err := w.declarePrincipal(name, roles); err != nil {
		fr.raise(ls, err)
	}
	ls.Push(fr.actor(ls, name))
	return 1
}

func (fr *fileRun) luaAs(ls *lua.LState) int {
	fr.world(ls, "as")
	user := ls.CheckString(1)
	if strings.TrimSpace(user) == "" {
		ls.ArgError(1, "name the user")
	}
	ls.Push(fr.actor(ls, user))
	return 1
}

func (fr *fileRun) luaFail(ls *lua.LState) int {
	fr.world(ls, "fail")
	fr.failNow(ls, "%s", ls.OptString(1, "t.fail"))
	return 0
}

func (fr *fileRun) luaClock(ls *lua.LState) int {
	w := fr.world(ls, "clock")
	s := ls.CheckString(1)
	at, err := parseInstant(s)
	if err != nil {
		ls.ArgError(1, err.Error())
	}
	if at.Year() < minClockYear || at.Year() > maxClockYear {
		ls.ArgError(1, fmt.Sprintf("the clock runs from %d to %d", minClockYear, maxClockYear))
	}
	w.clock.Store(at.UnixNano())
	return 0
}

// parseInstant reads an RFC 3339 instant or a YYYY-MM-DD date (midnight UTC).
func parseInstant(s string) (time.Time, error) {
	if at, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return at.UTC(), nil
	}
	if at, err := time.Parse(time.DateOnly, s); err == nil {
		return at, nil
	}
	return time.Time{}, fmt.Errorf("%q is not an RFC 3339 instant or a YYYY-MM-DD date", s)
}

func (fr *fileRun) luaAdvance(ls *lua.LState) int {
	w := fr.world(ls, "advance")
	secs := float64(ls.CheckNumber(1))
	if math.IsNaN(secs) || secs < 0 {
		ls.ArgError(1, "the clock only moves forward; use t.clock to set an earlier instant")
	}
	step := secs * float64(time.Second)
	if step >= float64(math.MaxInt64-w.clock.Load()) {
		ls.ArgError(1, fmt.Sprintf("the clock runs to %d", maxClockYear))
	}
	w.clock.Add(int64(step))
	return 0
}

func (fr *fileRun) luaNow(ls *lua.LState) int {
	w := fr.world(ls, "now")
	ls.Push(lua.LString(w.now().Format(time.RFC3339)))
	return 1
}

func (fr *fileRun) luaToday(ls *lua.LState) int {
	w := fr.world(ls, "today")
	ls.Push(lua.LString(w.now().Format(time.DateOnly)))
	return 1
}

// durationFn returns a function that converts a count of unit to seconds,
// the unit t.advance takes.
func durationFn(unit time.Duration) lua.LGFunction {
	return func(ls *lua.LState) int {
		n := float64(ls.CheckNumber(1))
		ls.Push(lua.LNumber(n * unit.Seconds()))
		return 1
	}
}

func (fr *fileRun) luaMail(ls *lua.LState) int {
	w := fr.world(ls, "mail")
	out := ls.NewTable()
	for _, m := range w.mail.Messages() {
		out.Append(mailTable(ls, m))
	}
	ls.Push(out)
	return 1
}

// fakeTable builds t.fake.
func (fr *fileRun) fakeTable(ls *lua.LState) *lua.LTable {
	t := ls.NewTable()
	str := func(name string, gen func(*faker) string) {
		t.RawSetString(name, ls.NewFunction(func(ls *lua.LState) int {
			w := fr.world(ls, "fake."+name)
			ls.Push(lua.LString(gen(w.fake)))
			return 1
		}))
	}
	str("name", func(g *faker) string { return g.f.Name() })
	str("email", func(g *faker) string { return g.f.Email() })
	str("sentence", func(g *faker) string { return g.f.Sentence() })
	str("word", func(g *faker) string { return g.f.Word() })
	t.RawSetString("date", ls.NewFunction(func(ls *lua.LState) int {
		w := fr.world(ls, "fake.date")
		within := dateSpan
		if n, ok := optionTable(ls, 1, "within").RawGetString("within").(lua.LNumber); ok {
			within = int(n)
		}
		if within < 0 {
			ls.ArgError(1, "within must not be negative")
		}
		day := w.now().AddDate(0, 0, w.fake.intRange(-within, within))
		ls.Push(lua.LString(day.Format(time.DateOnly)))
		return 1
	}))
	t.RawSetString("pick", ls.NewFunction(func(ls *lua.LState) int {
		w := fr.world(ls, "fake.pick")
		list := ls.CheckTable(1)
		n := list.Len()
		if n == 0 {
			ls.ArgError(1, "the list is empty")
		}
		ls.Push(list.RawGetInt(1 + w.fake.rng.IntN(n)))
		return 1
	}))
	t.RawSetString("value", ls.NewFunction(func(ls *lua.LState) int {
		w := fr.world(ls, "fake.value")
		fr.check(ls, w.ensure())
		v, err := w.fake.value(ls.CheckString(1), ls.CheckString(2))
		fr.check(ls, err)
		ls.Push(relalua.GoToLuaValue(ls, v))
		return 1
	}))
	t.RawSetString("entity", ls.NewFunction(func(ls *lua.LState) int {
		w := fr.world(ls, "fake.entity")
		fr.check(ls, w.ensure())
		props := map[string]any{}
		_, err := w.fake.fill(ls.CheckString(1), props)
		fr.check(ls, err)
		ls.Push(relalua.GoToLuaValue(ls, props))
		return 1
	}))
	return t
}
