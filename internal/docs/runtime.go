package docs

import (
	"context"
	"fmt"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	rlua "github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// buildTimeout bounds a single manual build. An island with an infinite loop
// (a flat `while true`, which the call-stack cap does not catch) is stopped
// only by this context deadline, so it must always be wired.
const buildTimeout = 30 * time.Second

// Options controls a manual build.
type Options struct {
	// Meta is the deployment's metamodel (loaded from the real project). It is
	// the source of every schema resolver's data.
	Meta *metamodel.Metamodel
	// Policy is the deployment's ACL policy for roles_matrix, or nil when the
	// project has no acl.yaml (roles_matrix then reports "no policy").
	Policy *acl.Policy
	// Strict promotes an empty resolve (a resolver / echo yielding nothing) from
	// a warning to a build failure.
	Strict bool
}

// docRuntime owns the per-build state the doc.* bindings close over: the real
// metamodel + policy (read), an ephemeral seeded memstore (read+write), the
// output buffer statement islands append to, and the strict flag.
type docRuntime struct {
	meta   *metamodel.Metamodel
	policy *acl.Policy
	store  *memstore.MemStore
	tracer tracer.Tracer
	strict bool

	rt  *rlua.Runtime
	out *strings.Builder // statement-island emit target
	ctx context.Context

	// warnings accumulates non-fatal issues (empty resolves in non-strict mode).
	warnings []string
}

// Build resolves every island in the manual source and returns the rendered
// Markdown. It is the package entry point.
func Build(ctx context.Context, src string, opts Options) (string, error) {
	if opts.Meta == nil {
		return "", fmt.Errorf("docs.Build: Meta is required")
	}
	segs, err := parse(src)
	if err != nil {
		return "", err
	}

	st := memstore.New()
	dr := &docRuntime{
		meta:   opts.Meta,
		policy: opts.Policy,
		store:  st,
		tracer: tracer.New(st),
		strict: opts.Strict,
		out:    &strings.Builder{},
		ctx:    ctx,
	}

	// A reader runtime gives us the sandbox (no io/os) plus rela.* read bindings;
	// we layer the doc.* module (emit + resolvers + raw-store seed) on top. The
	// seed writes go to the memstore directly, NOT through an entitymanager, so
	// no Mutator/WriteDeps machinery is needed.
	readDeps := rlua.ReadDeps{
		Store:  st,
		Tracer: dr.tracer,
		Meta:   opts.Meta,
	}
	rt := rlua.NewReader(readDeps, dr.out, rlua.WithContext(ctx), rlua.WithTimeout(buildTimeout))
	defer rt.Close()
	dr.rt = rt
	dr.registerModule()

	var b strings.Builder
	for _, seg := range segs {
		switch seg.kind {
		case segLiteral:
			b.WriteString(seg.body)
		case segStatement:
			out, err := dr.runStatement(seg)
			if err != nil {
				return "", err
			}
			b.WriteString(out)
		case segEcho:
			out, err := dr.runEcho(seg)
			if err != nil {
				return "", err
			}
			b.WriteString(out)
		}
	}
	return b.String(), nil
}

// Warnings returns the non-fatal issues accumulated during the last Build on
// this runtime (only meaningful in non-strict mode).
func (dr *docRuntime) Warnings() []string { return dr.warnings }

// runStatement executes one statement island. doc.* emit calls append to
// dr.out; we capture what this island produced and reset the buffer for the
// next one.
func (dr *docRuntime) runStatement(seg segment) (string, error) {
	dr.out.Reset()
	if err := dr.rt.RunString(seg.body); err != nil {
		return "", dr.wrapLuaErr(seg, err)
	}
	return dr.out.String(), nil
}

// runEcho evaluates an echo island's expression and returns its string value.
// The body is wrapped as `return <expr>` so the runtime yields a value.
func (dr *docRuntime) runEcho(seg segment) (string, error) {
	dr.out.Reset()
	val, err := dr.rt.RunActionString("return "+seg.body, "manual-echo")
	if err != nil {
		return "", dr.wrapLuaErr(seg, err)
	}
	s, err := coerceEcho(val)
	if err != nil {
		return "", &BuildError{Line: seg.line, Kind: "resolve", Msg: err.Error(), Snip: seg.body}
	}
	if s == "" {
		if dr.strict {
			return "", &BuildError{Line: seg.line, Kind: "strict", Msg: "echo island resolved to empty", Snip: seg.body}
		}
		dr.warnings = append(dr.warnings, fmt.Sprintf("manual:%d: echo resolved to empty: %s", seg.line, seg.body))
	}
	return s, nil
}

// coerceEcho converts an echo expression's Go value to its inline string form.
// strings pass through; numbers are formatted; nil is empty; bool/table/other
// are author errors (a block resolver placed in an inline span).
func coerceEcho(v any) (string, error) {
	switch t := v.(type) {
	case nil:
		return "", nil
	case string:
		return t, nil
	case int64:
		return fmt.Sprint(t), nil
	case float64:
		// Whole floats print without a trailing ".0" for tidy counts.
		if t == float64(int64(t)) {
			return fmt.Sprint(int64(t)), nil
		}
		return fmt.Sprint(t), nil
	case bool:
		return "", fmt.Errorf("echo island returned a boolean; inline spans need a string or number")
	default:
		return "", fmt.Errorf("echo island returned %T; inline spans need a string or number (did you mean a ```rela block?)", v)
	}
}

// wrapLuaErr turns a runtime error into a BuildError anchored to the manual
// line. The Lua frame line is intra-island (1-based within the island body);
// the manual line is the island's start line plus that offset minus one.
func (dr *docRuntime) wrapLuaErr(seg segment, err error) error {
	// A BuildError raised by a resolver already carries the right manual line.
	if be, ok := err.(*BuildError); ok {
		return be
	}
	line := seg.line
	if frames := dr.rt.ErrorFrames(); len(frames) > 0 {
		if fl := frames[len(frames)-1].Line; fl > 0 {
			line = seg.line + fl - 1
		}
	}
	return &BuildError{Line: line, Kind: "lua", Msg: err.Error(), Snip: strings.TrimSpace(seg.body)}
}

// luaFail raises a BuildError from inside a doc.* binding, anchored to the
// currently-executing island. gopher-lua unwinds via RaiseError; the message
// is recovered by the runtime's error capture and surfaced through wrapLuaErr.
func (dr *docRuntime) luaFail(ls *lua.LState, kind, format string, args ...any) int {
	ls.RaiseError("%s", &BuildError{Kind: kind, Msg: fmt.Sprintf(format, args...)})
	return 0
}
