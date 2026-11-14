package lua

import (
	"errors"
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// Resource bounds of a Lua state.
//
// gopher-lua has no allocator hook, so a precise per-state memory limit is
// not possible. These bounds apply in every mode:
//
//   - The call stack holds at most luaCallStackSize frames. Deeper
//     recursion raises "stack overflow".
//   - The value stack (gopher-lua's registry) starts at
//     luaRegistryInitial slots and grows to at most luaRegistryMax.
//     Pushing past it raises "registry overflow". This bounds unpack(),
//     varargs and table.concat over huge tables. Starting small keeps an
//     idle state at about 64 KiB of stack instead of 1 MiB.
//   - string.rep, string.format and table.concat refuse a result larger
//     than maxBuiltString before they allocate it. These are the builders
//     that turn a small input into a large output in one call.
//   - A script runs under the runtime's timeout (DefaultTimeout), which
//     gopher-lua checks between instructions.
//
// What is NOT bounded beyond the timeout:
//
//   - The `..` operator. It is a VM instruction with no hook, so a loop
//     that doubles a string (s = s .. s) reaches gigabytes in a few dozen
//     steps, well inside the timeout.
//   - string.gsub with a replacement that expands captures, and the other
//     string functions, whose result is linear in their inputs but whose
//     inputs `..` can make large.
//   - The number and size of tables. A loop that builds many tables is
//     bounded only by the timeout.
//
// Those allocations reach the Go heap of the whole process, so they are
// bounded only by the process memory limit (GOMEMLIMIT, the container's
// limit). A sampled heap watchdog was rejected: the heap is shared by every
// tenant, so it would cancel whichever script happened to run when another
// one grew it.
const (
	luaCallStackSize   = 1024
	luaRegistryInitial = 4 * 1024
	luaRegistryMax     = 64 * 1024
	maxBuiltString     = 16 << 20 // 16 MiB
	bytesPerMiB        = 1 << 20

	// maxFormatVerbBytes bounds what one string.format verb adds besides
	// its string argument: a width or precision of at most 99, and a %f of
	// 1e308 (309 digits) plus a precision of 99.
	maxFormatVerbBytes = 512
)

// newBoundedState returns a Lua state with no libraries opened and the
// stack bounds above.
func newBoundedState() *lua.LState {
	return lua.NewState(lua.Options{
		SkipOpenLibs:     true,
		CallStackSize:    luaCallStackSize,
		RegistrySize:     luaRegistryInitial,
		RegistryMaxSize:  luaRegistryMax,
		RegistryGrowStep: lua.RegistryGrowStep,
	})
}

// installBuilderCaps replaces string.rep, string.format and table.concat
// with versions that refuse a result over maxBuiltString. It must run after
// the string and table libraries are open. The string table is also the
// __index of the string metatable, so ("x"):rep(n) is covered too.
func installBuilderCaps(ls *lua.LState) {
	if str, ok := ls.GetGlobal(lua.StringLibName).(*lua.LTable); ok {
		ls.SetField(str, "rep", ls.NewFunction(boundedStrRep))
		ls.SetField(str, "format", ls.NewFunction(boundedStrFormat))
	}
	if tbl, ok := ls.GetGlobal(lua.TabLibName).(*lua.LTable); ok {
		if orig, isFn := ls.GetField(tbl, "concat").(*lua.LFunction); isFn {
			ls.SetField(tbl, "concat", ls.NewFunction(boundedTableConcat(orig)))
		}
	}
}

func raiseTooLarge(ls *lua.LState, fn string) {
	ls.RaiseError("%s: result would exceed the %d MiB string limit", fn, maxBuiltString/bytesPerMiB)
}

// boundedStrRep is string.rep(s, n) with the result capped.
func boundedStrRep(ls *lua.LState) int {
	s := ls.CheckString(1)
	n := ls.CheckInt(2)
	if n <= 0 || s == "" {
		ls.Push(lua.LString(""))
		return 1
	}
	if n > maxBuiltString/len(s) {
		raiseTooLarge(ls, "string.rep")
		return 0
	}
	ls.Push(lua.LString(strings.Repeat(s, n)))
	return 1
}

// boundedStrFormat is string.format with Lua 5.1 format syntax and the
// result capped.
//
// gopher-lua passes the whole format to Go's fmt.Sprintf. Go's fmt accepts
// explicit argument indexes (%[1]s) and a * width, so one large argument
// could be used many times and no size estimate over the arguments would
// hold. This version parses the format itself and formats verb by verb,
// with one argument each. Each argument is used at most once. A verb is
// formatted with fmt.Sprintf on the same Lua value as before, so the output
// of a valid format does not change.
//
// The whole result is estimated before anything is built, and the running
// size is checked again as each verb is added.
func boundedStrFormat(ls *lua.LState) int {
	f := ls.CheckString(1)
	verbs, err := parseLuaFormat(f)
	if err != nil {
		ls.RaiseError("string.format: %s", err.Error())
		return 0
	}
	top := ls.GetTop()
	argOf := func(v formatVerb) lua.LValue {
		if v.arg < 0 || v.arg > top {
			return nil
		}
		return ls.Get(v.arg)
	}
	size := len(f)
	for _, v := range verbs {
		size += v.estimate(argOf(v))
		if size > maxBuiltString {
			raiseTooLarge(ls, "string.format")
			return 0
		}
	}
	var b strings.Builder
	lit := 0
	for _, v := range verbs {
		b.WriteString(f[lit:v.start])
		lit = v.end
		switch arg := argOf(v); {
		case v.arg < 0:
			b.WriteByte('%')
		case arg == nil:
			ls.ArgError(v.arg, "no value")
			return 0
		default:
			fmt.Fprintf(&b, v.spec, arg)
		}
		if b.Len() > maxBuiltString {
			raiseTooLarge(ls, "string.format")
			return 0
		}
	}
	b.WriteString(f[lit:])
	ls.Push(lua.LString(b.String()))
	return 1
}

// formatVerb is one conversion of a Lua format string. f[start:end] is the
// conversion as written; spec is the Go format for it. arg is the stack
// index of its argument, or -1 for "%%", which takes none.
type formatVerb struct {
	start, end int
	arg        int
	spec       string
	verb       byte
}

// estimate bounds the bytes this verb adds for arg. A width or precision is
// at most 99 and a %f of 1e308 has 309 digits, so a verb adds at most
// maxFormatVerbBytes besides its string argument. %q escapes a byte in at
// most four bytes.
func (v formatVerb) estimate(arg lua.LValue) int {
	s, isStr := arg.(lua.LString)
	switch {
	case v.arg < 0:
		return 0
	case !isStr:
		return maxFormatVerbBytes
	case v.verb == 'q':
		return maxFormatVerbBytes + 4*len(s)
	default:
		return maxFormatVerbBytes + len(s)
	}
}

const (
	luaFormatFlags = "-+ #0"
	luaFormatVerbs = "cdiouxXeEfgGqs"
)

// parseLuaFormat splits f into its conversions with the Lua 5.1 rules:
// flags from "-+ #0", a width and a precision of at most two digits each,
// and one of the verbs cdiouxXeEfgGqs. "%%" is a literal percent sign and is
// returned as a verb that takes no argument. An explicit argument index
// ("[") and a "*" width are refused.
func parseLuaFormat(f string) ([]formatVerb, error) {
	var verbs []formatVerb
	nextArg := 2 // stack index 1 is the format itself
	for i := 0; i < len(f); i++ {
		if f[i] != '%' {
			continue
		}
		start := i
		i++
		if i < len(f) && f[i] == '%' {
			verbs = append(verbs, formatVerb{start: start, end: i + 1, arg: -1, spec: "%%", verb: '%'})
			continue
		}
		flagStart := i
		for i < len(f) && strings.IndexByte(luaFormatFlags, f[i]) >= 0 {
			i++
		}
		if i-flagStart >= len(luaFormatFlags) {
			return nil, errors.New("invalid format (repeated flags)")
		}
		if digitsAt(f, &i) > 2 {
			return nil, errors.New("invalid format (width or precision too long)")
		}
		if i < len(f) && f[i] == '.' {
			i++
			if digitsAt(f, &i) > 2 {
				return nil, errors.New("invalid format (width or precision too long)")
			}
		}
		if i >= len(f) {
			return nil, errors.New("invalid format (ends in a conversion)")
		}
		c := f[i]
		if c == '[' || c == '*' {
			return nil, fmt.Errorf("invalid format (%q is not supported)", c)
		}
		if strings.IndexByte(luaFormatVerbs, c) < 0 {
			return nil, fmt.Errorf("invalid option '%%%c' to 'format'", c)
		}
		goVerb := c
		if c == 'u' {
			// Go has no %u. An unsigned Lua 5.1 conversion of a
			// non-negative number prints as %d.
			goVerb = 'd'
		}
		spec := f[start:i] + string(goVerb)
		verbs = append(verbs, formatVerb{start: start, end: i + 1, arg: nextArg, spec: spec, verb: c})
		nextArg++
	}
	return verbs, nil
}

// digitsAt advances *i past a run of decimal digits and returns its length.
func digitsAt(f string, i *int) int {
	start := *i
	for *i < len(f) && f[*i] >= '0' && f[*i] <= '9' {
		*i++
	}
	return *i - start
}

// boundedTableConcat wraps table.concat(t, sep, i, j). It sums the length of
// the result first, with the same index rules as gopher-lua, and leaves
// argument and element errors to the original.
func boundedTableConcat(orig *lua.LFunction) lua.LGFunction {
	return func(ls *lua.LState) int {
		tbl := ls.CheckTable(1)
		sep := ls.OptString(2, "")
		n := tbl.Len()
		const firstArg, lastArg = 3, 4
		i := max(min(ls.OptInt(firstArg, 1), n), 1)
		j := min(ls.OptInt(lastArg, n), n)
		size := 0
		for k := i; k <= j; k++ {
			switch v := tbl.RawGetInt(k).(type) {
			case lua.LString:
				size += len(v)
			case lua.LNumber:
				size += len(v.String())
			}
			if k != j {
				size += len(sep)
			}
			if size > maxBuiltString {
				raiseTooLarge(ls, "table.concat")
				return 0
			}
		}
		return callOriginal(ls, orig)
	}
}

// callOriginal calls orig with this call's arguments and returns its one
// result.
func callOriginal(ls *lua.LState, orig *lua.LFunction) int {
	top := ls.GetTop()
	ls.Push(orig)
	for i := 1; i <= top; i++ {
		ls.Push(ls.Get(i))
	}
	ls.Call(top, 1)
	return 1
}
