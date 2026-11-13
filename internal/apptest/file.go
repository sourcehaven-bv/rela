package apptest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	relalua "github.com/Sourcehaven-BV/rela/internal/lua"
)

// testCase is one t.case declaration.
type testCase struct {
	name string
	fn   *lua.LFunction
	// where is the file:line of the t.case call.
	where string
}

// fileRun is one test file being run: its Lua state, its declared cases,
// and the case running now.
type fileRun struct {
	r     *runner
	file  string
	ls    *lua.LState
	cases []testCase
	// current is the world of the case running now; nil at the top level.
	current *world
	// rng serves math.random and t.random: the file's own at the top level,
	// the case's during a case.
	rng *rand.Rand
}

// runFile runs one file and adds its results to rep. Each case runs on its
// own Lua state: the file's top level runs again before it, so globals, the
// module cache and actors a case leaves behind cannot reach the next case,
// and a case gives the same result whether it runs alone or with the rest.
func (r *runner) runFile(ctx context.Context, file string, rep *Report) {
	start := time.Now()
	fail := func(where, msg string) {
		rep.add(CaseResult{File: file, Location: where, Message: msg, Duration: time.Since(start)})
	}
	code, err := r.files.Load(ctx, file)
	if err != nil {
		fail("", fmt.Sprintf("read %s: %v", file, err))
		return
	}
	fr, where, msg := r.declare(ctx, file, string(code))
	if fr == nil {
		fail(where, msg)
		return
	}
	cases := fr.cases
	fr.ls.Close()
	if len(cases) == 0 {
		fail("", "the file declares no cases; declare them with t.case(name, function() ... end)")
		return
	}
	for i, c := range cases {
		if r.filter != nil && !r.filter.MatchString(c.name) {
			continue
		}
		rep.add(r.runCaseFresh(ctx, file, string(code), i, c))
	}
}

// runCaseFresh runs the i-th case of file, c, on a new Lua state.
func (r *runner) runCaseFresh(ctx context.Context, file, code string, i int, c testCase) CaseResult {
	start := time.Now()
	fr, where, msg := r.declare(ctx, file, code)
	if fr == nil {
		return CaseResult{File: file, Case: c.name, Location: where, Message: msg, Duration: time.Since(start)}
	}
	defer fr.ls.Close()
	if i >= len(fr.cases) || fr.cases[i].name != c.name {
		return CaseResult{File: file, Case: c.name, Location: c.where, Duration: time.Since(start),
			Message: "the file declared different cases when it ran again; declare cases the same way on every run"}
	}
	return fr.runCase(ctx, fr.cases[i])
}

// declare runs file's top level on a new Lua state and returns the run with
// its declared cases. On a compile or top-level error it returns nil and
// the error's location and message. The caller closes the state.
func (r *runner) declare(ctx context.Context, file, code string) (fr *fileRun, where, msg string) {
	fr = &fileRun{r: r, file: file, rng: seededRand(r.caseSeed(file, ""))}
	fr.ls = relalua.NewSandbox(r.files, []string{TestsDir}, map[string]func(*lua.LState) lua.LValue{
		"rela.test": fr.module,
	})
	fr.replaceMathRandom()

	topCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	fr.ls.SetContext(topCtx)
	defer fr.ls.RemoveContext()
	fn, err := fr.ls.Load(strings.NewReader(code), file)
	if err != nil {
		fr.ls.Close()
		where, msg = splitLuaError(err.Error())
		return nil, where, "compile: " + msg
	}
	fr.ls.Push(fn)
	if err := fr.ls.PCall(0, 0, nil); err != nil {
		fr.ls.Close()
		where, msg = splitLuaError(luaErrorText(err))
		return nil, where, msg
	}
	return fr, "", ""
}

// runCase runs one case on a fresh world.
func (fr *fileRun) runCase(ctx context.Context, c testCase) CaseResult {
	start := time.Now()
	seed := fr.r.caseSeed(fr.file, c.name)
	w := newWorld(fr.r, seed)
	fr.current = w
	fr.rng = w.rng
	defer func() { fr.current = nil }()

	caseCtx, cancel := context.WithTimeout(ctx, fr.r.timeout)
	defer cancel()
	w.ctx = caseCtx
	fr.ls.SetContext(caseCtx)
	fr.ls.Push(c.fn)
	err := fr.ls.PCall(0, 0, nil)
	fr.ls.RemoveContext()
	closeErr := w.close()

	res := CaseResult{File: fr.file, Case: c.name, OK: true}
	switch {
	case w.failure == nil && err == nil && closeErr != nil:
		res.OK, res.Message = false, "close the test world: "+closeErr.Error()
	case w.failure == nil && err == nil && w.network.Load():
		res.OK, res.Message = false, errRealNetwork.Error()+"; the case's code tried one and went on"
	case w.failure != nil:
		res.OK, res.Location, res.Message = false, w.failure.where, w.failure.msg
	case err != nil:
		res.OK = false
		res.Location, res.Message = splitLuaError(luaErrorText(err))
		if errors.Is(caseCtx.Err(), context.DeadlineExceeded) {
			res.Message = fmt.Sprintf("the case ran longer than %s", fr.r.timeout)
		}
	}
	res.Duration = time.Since(start)
	return res
}

// seededRand returns a generator drawn from seed.
func seededRand(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, ^seed)) //nolint:gosec // reproducible test data, not secrets
}

// replaceMathRandom points math.random at the run's seeded generator and
// makes math.randomseed refuse, so no file can reseed it.
func (fr *fileRun) replaceMathRandom() {
	m, ok := fr.ls.GetGlobal("math").(*lua.LTable)
	if !ok {
		return
	}
	fr.ls.SetField(m, "random", fr.ls.NewFunction(fr.luaRandom))
	fr.ls.SetField(m, "randomseed", fr.ls.NewFunction(func(ls *lua.LState) int {
		ls.RaiseError("math.randomseed is not available in rela test; pass --seed to the command instead")
		return 0
	}))
}

// luaRandom is math.random and t.random: no argument gives a float in
// [0, 1), one gives an integer in [1, m], two give one in [m, n].
func (fr *fileRun) luaRandom(ls *lua.LState) int {
	switch ls.GetTop() {
	case 0:
		ls.Push(lua.LNumber(fr.rng.Float64()))
	case 1:
		hi := ls.CheckInt(1)
		if hi < 1 {
			ls.ArgError(1, "interval is empty")
		}
		ls.Push(lua.LNumber(1 + fr.rng.IntN(hi)))
	default:
		lo, hi := ls.CheckInt(1), ls.CheckInt(2)
		if hi < lo {
			ls.ArgError(2, "interval is empty")
		}
		if hi-lo < 0 || hi-lo == math.MaxInt {
			ls.ArgError(2, "interval is too large")
		}
		ls.Push(lua.LNumber(lo + fr.rng.IntN(hi-lo+1)))
	}
	return 1
}

// luaErrorText is the message of an error from PCall, without gopher-lua's
// stack trace.
func luaErrorText(err error) string {
	var ae *lua.ApiError
	if errors.As(err, &ae) && ae.Object != nil {
		return ae.Object.String()
	}
	return err.Error()
}

// luaPosition matches the "file:line:" a Lua error message starts with.
var luaPosition = regexp.MustCompile(`^([^\s:][^:]*):(\d+):\s*`)

// splitLuaError splits "file:line: message" into the location and the
// message. A message without a position comes back whole.
func splitLuaError(s string) (where, msg string) {
	m := luaPosition.FindStringSubmatch(s)
	if m == nil {
		return "", s
	}
	if _, err := strconv.Atoi(m[2]); err != nil {
		return "", s
	}
	return m[1] + ":" + m[2], s[len(m[0]):]
}

// where returns the file:line of the Lua code that called the running Go
// function.
func where(ls *lua.LState) string {
	return strings.TrimSuffix(ls.Where(1), ":")
}
