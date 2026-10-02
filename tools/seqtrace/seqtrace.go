// Package seqtrace is the runtime half of the call tracer behind
// `just seqtrace-*`. Code never imports it by hand: the rewriter in
// tools/seqtrace/rewrite injects calls to it into a copy of the source, and
// `go build -overlay` compiles that copy. A normal build never links it.
//
// Each goroutine keeps a stack of call IDs. A call's parent is the frame
// below it on the same goroutine. When a goroutine's stack is empty, the
// parent is found in one of four ways, recorded as the call's "via":
//
//   - "ctx": the context.Context parameter carries a call ID from another
//     goroutine. The rewriter stores the current call ID in every named ctx
//     parameter, so this covers work handed over in a message that carries a
//     ctx. It also wins over a non-empty stack, which is how a long-lived
//     worker loop attributes each job to the request that queued it.
//   - "go": the goroutine was started by a rewritten go statement.
//   - "closure": a function literal runs on a goroutine that library code
//     started (errgroup, for example) while the call that created the
//     literal is still running.
//   - "" with no parent: the call is a root.
//
// Tracing is off unless SEQTRACE_OUT names an output file. SEQTRACE_ROOT is
// an optional regexp on "pkg.Func"; when set, a parentless call that does not
// match it is not recorded, and neither is anything it calls synchronously.
// That keeps startup and background loops out of a trace of HTTP requests.
package seqtrace

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Via values recorded on a call whose parent runs on another goroutine.
const (
	ViaCtx     = "ctx"
	ViaGo      = "go"
	ViaClosure = "closure"
)

// Event is one line of the trace file.
type Event struct {
	Kind   string `json:"e"`           // "c" for a call, "r" for its return
	ID     uint64 `json:"id"`          // call ID, unique within one process
	Parent uint64 `json:"p,omitempty"` // zero for a root
	Via    string `json:"v,omitempty"` // how the parent was found, see package doc
	G      uint64 `json:"g,omitempty"` // goroutine ID
	Pkg    string `json:"pkg,omitempty"`
	Fn     string `json:"fn,omitempty"`
	T      int64  `json:"t"` // nanoseconds since the process started tracing
	// Args summarizes the parameters of a call as "name=value", Results the
	// results of a return; see summarize.go.
	Args    []string `json:"a,omitempty"`
	Results []string `json:"rv,omitempty"`
}

// Frame is the handle Enter returns; the injected code defers its Exit.
type Frame struct {
	id uint64
	g  uint64
	on bool   // a stack entry was pushed, so Exit must pop it
	fn string // for redaction of results
}

type ctxKey struct{}

type ctxVal struct{ id, g uint64 }

type pendingSpawn struct {
	parent uint64
	at     time.Time
}

type gstate struct {
	stack []uint64 // 0 marks an untraced frame
	// muted is non-zero while the tracer summarizes values on this
	// goroutine. Summarizing calls Error methods, which may be instrumented
	// code; their calls are tracer noise, not part of the traced program.
	muted int
}

// spawnTTL bounds how long a go statement waits to be claimed. A spawn of a
// function that is not instrumented is never claimed and must not match a
// later, unrelated call of the same name.
const spawnTTL = 2 * time.Second

const (
	traceMode  = 0o600 // a trace holds unredacted data; see the README
	bufSize    = 1 << 20
	flushEvery = 250 * time.Millisecond
)

var (
	enabled bool
	rootRe  *regexp.Regexp
	start   time.Time
	nextID  atomic.Uint64

	mu      sync.Mutex
	out     *bufio.Writer
	gs      = map[uint64]*gstate{}
	active  = map[uint64]struct{}{}
	pending = map[string][]pendingSpawn{}
)

func init() {
	path := os.Getenv("SEQTRACE_OUT")
	if path == "" {
		return
	}
	if r := os.Getenv("SEQTRACE_ROOT"); r != "" {
		re, err := regexp.Compile(r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "seqtrace: bad SEQTRACE_ROOT: %v\n", err)
			return
		}
		rootRe = re
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, traceMode) //nolint:gosec // operator-chosen path
	if err != nil {
		fmt.Fprintf(os.Stderr, "seqtrace: %v\n", err)
		return
	}
	out = bufio.NewWriterSize(f, bufSize)
	start = time.Now()
	enabled = true
	fmt.Fprintf(os.Stderr, "seqtrace: writing %s\n", path)
	// The process usually ends by signal, so flush on a timer rather than
	// relying on an exit hook.
	go func() {
		for range time.Tick(flushEvery) {
			mu.Lock()
			_ = out.Flush()
			mu.Unlock()
		}
	}()
}

// Flush writes buffered events. Tests that exit before the flush timer fires
// can call it; the rewriter never injects it.
func Flush() {
	if !enabled {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	_ = out.Flush()
}

// Enter records a call of a declared function or method. args alternates
// parameter names and values.
func Enter(ctx context.Context, pkg, fn string, args ...any) Frame {
	if !enabled {
		return Frame{}
	}
	return enter(ctx, pkg, fn, 0, "", args)
}

// EnterClosure records a call of a function literal. creator is the call that
// evaluated the literal, captured by Current at that moment. goStmt is true
// when the literal is the function of a go statement.
func EnterClosure(ctx context.Context, creator uint64, goStmt bool, pkg, fn string, args ...any) Frame {
	if !enabled {
		return Frame{}
	}
	via := ViaClosure
	if goStmt {
		via = ViaGo
	}
	return enter(ctx, pkg, fn, creator, via, args)
}

// Current returns the call ID running on this goroutine, or zero.
func Current() uint64 {
	if !enabled {
		return 0
	}
	g := gid()
	mu.Lock()
	defer mu.Unlock()
	if st := gs[g]; st != nil && len(st.stack) > 0 {
		return st.stack[len(st.stack)-1]
	}
	return 0
}

// Spawn is injected before `go f(...)` when f is not a function literal.
// The first call named name on a goroutine with an empty stack claims it.
func Spawn(name string) {
	if !enabled {
		return
	}
	p := Current()
	if p == 0 {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	// Drop expired entries here too: a spawn whose goroutine finds its
	// parent through ctx never reaches claimSpawn.
	q := pending[name]
	for len(q) > 0 && time.Since(q[0].at) > spawnTTL {
		q = q[1:]
	}
	pending[name] = append(q, pendingSpawn{parent: p, at: time.Now()})
}

// Ctx returns ctx carrying this frame's call ID. The rewriter assigns the
// result back to the function's ctx parameter.
func (f Frame) Ctx(ctx context.Context) context.Context {
	if f.id == 0 || ctx == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, ctxVal{id: f.id, g: f.g})
}

// Exit records the return of the call Enter recorded, with its results.
func (f Frame) Exit(results ...any) {
	if !f.on {
		return
	}
	var rv []string
	if f.id != 0 && len(results) > 0 {
		setMuted(f.g, 1)
		rv = summarizeResults(f.fn, results)
		setMuted(f.g, -1)
	}
	mu.Lock()
	defer mu.Unlock()
	st := gs[f.g]
	if st == nil || len(st.stack) == 0 {
		return
	}
	st.stack = st.stack[:len(st.stack)-1]
	if len(st.stack) == 0 {
		delete(gs, f.g)
	}
	if f.id != 0 {
		delete(active, f.id)
		write(Event{Kind: "r", ID: f.id, T: int64(time.Since(start)), Results: rv})
		if len(st.stack) == 0 {
			// The goroutine's outermost call returned: flush, so a process
			// that exits before the timer fires loses nothing.
			_ = out.Flush()
		}
	}
}

func enter(ctx context.Context, pkg, fn string, creator uint64, closureVia string, args []any) Frame {
	g := gid()
	var cp ctxVal
	if ctx != nil {
		cp, _ = ctx.Value(ctxKey{}).(ctxVal)
	}
	f, e := decide(g, cp, pkg, fn, creator, closureVia)
	if f.id == 0 {
		return f
	}
	// Summarize outside the lock: an Error method may re-enter the tracer.
	if len(args) > 0 {
		setMuted(g, 1)
		e.Args = summarizeArgs(fn, args)
		setMuted(g, -1)
	}
	mu.Lock()
	write(e)
	mu.Unlock()
	return f
}

func setMuted(g uint64, delta int) {
	mu.Lock()
	defer mu.Unlock()
	if st := gs[g]; st != nil {
		st.muted += delta
	}
}

// decide pushes a stack entry for the call and, when it is recorded,
// returns its event. A zero Frame.id means the call is not recorded.
func decide(g uint64, cp ctxVal, pkg, fn string, creator uint64, closureVia string) (Frame, Event) {
	mu.Lock()
	defer mu.Unlock()
	st := gs[g]
	if st == nil {
		st = &gstate{}
		gs[g] = st
	}
	if st.muted > 0 {
		st.stack = append(st.stack, 0)
		return Frame{g: g, on: true}, Event{}
	}
	var top uint64
	if n := len(st.stack); n > 0 {
		top = st.stack[n-1]
	}

	var parent uint64
	var via string
	switch {
	case cp.id != 0 && cp.g != g:
		parent, via = cp.id, ViaCtx
	case top != 0:
		parent = top
	case len(st.stack) > 0:
		// Inside an untraced frame: only a root match starts tracing.
	case closureVia == ViaGo && creator != 0:
		parent, via = creator, ViaGo
	case closureVia == ViaClosure && creator != 0 && isActive(creator):
		parent, via = creator, ViaClosure
	default:
		if p := claimSpawn(fn); p != 0 {
			parent, via = p, ViaGo
		}
	}

	name := pkg + "." + fn
	if parent == 0 && rootRe != nil && !rootRe.MatchString(name) {
		st.stack = append(st.stack, 0)
		return Frame{g: g, on: true}, Event{}
	}

	id := nextID.Add(1)
	st.stack = append(st.stack, id)
	active[id] = struct{}{}
	e := Event{Kind: "c", ID: id, Parent: parent, Via: via, G: g, Pkg: pkg, Fn: fn, T: int64(time.Since(start))}
	return Frame{id: id, g: g, on: true, fn: fn}, e
}

func isActive(id uint64) bool {
	_, ok := active[id]
	return ok
}

// claimSpawn pops the oldest unexpired go statement waiting for fn. Callers
// hold mu.
func claimSpawn(fn string) uint64 {
	name := fn
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[i+1:]
	}
	q := pending[name]
	for len(q) > 0 && time.Since(q[0].at) > spawnTTL {
		q = q[1:]
	}
	if len(q) == 0 {
		delete(pending, name)
		return 0
	}
	p := q[0].parent
	if len(q) == 1 {
		delete(pending, name)
	} else {
		pending[name] = q[1:]
	}
	return p
}

// write appends one event. Callers hold mu.
func write(e Event) {
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	_, _ = out.Write(b)
	_ = out.WriteByte('\n')
}

// gid parses the goroutine ID from the first line of runtime.Stack:
// "goroutine 123 [running]:". Slow next to a real call, fast enough for a
// diagnostic build.
func gid() uint64 {
	var buf [64]byte
	b := buf[:runtime.Stack(buf[:], false)]
	b = b[len("goroutine "):]
	if i := bytes.IndexByte(b, ' '); i >= 0 {
		b = b[:i]
	}
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
