package docs

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	rlua "github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// buildTimeout bounds a whole manual build (all islands together, via a child
// context deadline in Build). An island with an infinite loop
// (a flat `while true`, which the call-stack cap does not catch) is stopped
// only by this context deadline, so it must always be wired.
const buildTimeout = 30 * time.Second

// screenshotBuildTimeout is the wider ceiling for a build that captures
// screenshots — browser launch + per-island navigate/capture need more than the
// Tier-A 30s.
const screenshotBuildTimeout = 5 * time.Minute

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

	// Capturer renders screenshot{} islands (Tier B). Injected by the CLI so the
	// core docs package stays browser-free. nil ⇒ screenshot{} fails loud.
	Capturer Capturer
	// CapturerErr is the reason a Capturer could not be built (e.g. no Chrome);
	// surfaced in the screenshot{} fail-loud message when Capturer is nil.
	CapturerErr string
	// ProjectDir is the documented project root; screenshot{} copies its schema/
	// config into the temp project the SPA renders.
	ProjectDir string
	// OutDir is where screenshot{} writes PNGs (derived from --out).
	OutDir string
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
	// ctx is stored because the doc.* bindings are gopher-lua callbacks
	// (func(*lua.LState) int) that cannot take a context parameter; the runtime
	// is short-lived and request-scoped, mirroring lua.Runtime.parentCtx.
	ctx context.Context //nolint:containedctx // request-scoped Lua-binding callbacks

	// pending holds the typed BuildError a doc.* binding raised on the current
	// island. gopher-lua's RaiseError only carries a string, so we stash the
	// typed error here and re-attach it in wrapLuaErr rather than round-tripping
	// it through the Lua error channel (which would lose the type + line).
	pending *BuildError

	// seedCounts is a per-type auto-id counter for the seed's mintID, avoiding a
	// full-store scan per create().
	seedCounts map[string]int

	// seedOps records every create/link so a screenshot{} island can replay them
	// against a fresh fsstore temp project (DR-S2).
	seedOps []SeedOp

	// capturer renders screenshot{} islands. nil ⇒ screenshot{} fails loud (the
	// Tier-B browser dependency is injected only by the CLI, keeping core docs
	// browser-free). See screenshot.go.
	capturer Capturer
	// capturerErr is the reason the capturer could not be constructed (e.g. no
	// Chrome), surfaced in the fail-loud message when a screenshot{} needs it.
	capturerErr string

	// projectDir is the documented project's root (schema/config copied into the
	// screenshot temp project). Empty in a schema-only build.
	projectDir string

	// outDir is where screenshot{} PNGs are written (derived from the build's
	// --out); empty ⇒ PNGs written next to the cwd and referenced by basename.
	outDir string

	// warnings accumulates non-fatal issues (empty resolves in non-strict mode).
	warnings []string
}

// Build resolves every island in the manual source and returns the rendered
// Markdown. It is the package entry point.
func Build(ctx context.Context, src string, opts Options) (string, error) {
	if opts.Meta == nil {
		return "", errors.New("docs.Build: Meta is required")
	}
	segs, err := parse(src)
	if err != nil {
		return "", err
	}

	// Bound the WHOLE build, not each island: the runtime's per-run applyTimeout
	// resets a fresh deadline on every island, so without a build-wide deadline
	// N islands give no effective ceiling. A child ctx deadline caps the total.
	// If the caller already set an earlier deadline, that one wins. Screenshot
	// builds get a longer ceiling (browser launch + navigate + capture).
	deadline := buildTimeout
	if opts.Capturer != nil {
		deadline = screenshotBuildTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	// The Capturer owns the temp project + server + browser; tear it down
	// unconditionally so a panic mid-build can't leak them (DR-M3).
	if opts.Capturer != nil {
		defer func() { _ = opts.Capturer.Close() }()
	}

	st := memstore.New()
	dr := &docRuntime{
		meta:        opts.Meta,
		policy:      opts.Policy,
		store:       st,
		tracer:      tracer.New(st),
		strict:      opts.Strict,
		out:         &strings.Builder{},
		ctx:         ctx,
		capturer:    opts.Capturer,
		capturerErr: opts.CapturerErr,
		projectDir:  opts.ProjectDir,
		outDir:      opts.OutDir,
	}

	// A reader runtime gives us the sandbox (no io/os) plus rela.* read bindings;
	// we layer the doc.* module (emit + resolvers + raw-store seed) on top. The
	// seed writes go to the memstore directly, NOT through an entitymanager, so
	// no Mutator/WriteDeps machinery is needed.
	// Unrestricted reads: this runtime reads a throwaway memstore the doc
	// build just seeded itself, and runs at the operator trust boundary
	// (whoever builds the docs already has the project). No ACL applies.
	readDeps := rlua.ReadDeps{
		VisibleReader: visibility.Unrestricted(st),
		Tracer:        dr.tracer,
		Meta:          opts.Meta,
	}
	rt := rlua.NewReader(readDeps, dr.out, rlua.WithContext(ctx), rlua.WithTimeout(buildTimeout),
		// The docs build runs from the operator shell / CI over in-repo
		// scripts, the same trust boundary as `rela script` (TKT-YH52OM).
		rlua.WithCapabilities(rlua.TrustedCapabilities()))
	defer rt.Close()
	dr.rt = rt
	dr.registerModule()

	var sw seamWriter
	for _, seg := range segs {
		switch seg.kind {
		case segLiteral:
			sw.write(seg.body)
		case segStatement:
			out, err := dr.runStatement(seg) //nolint:contextcheck // runtime ctx bound at construction via WithContext
			if err != nil {
				return "", err
			}
			sw.write(out)
		case segEcho:
			out, err := dr.runEcho(seg) //nolint:contextcheck // runtime ctx bound at construction via WithContext
			if err != nil {
				return "", err
			}
			sw.write(out)
		}
	}
	return sw.result(), nil
}

// seamWriter assembles the rendered segments while enforcing exactly one
// invariant: at most one blank line at any *seam* between two segments. It caps
// the blank run that straddles a boundary — the trailing blanks of what was
// already written plus the leading blanks of the next segment — to a single
// blank line, and trims trailing blanks at EOF to one final newline.
//
// Crucially it never touches a segment's *interior*: a resolver that emits a
// trailing blank (h1/md/roles_matrix) and a manual author who leaves a blank
// after a fence would otherwise combine into a double blank (MD012), yet a
// ```markdown literal sample with intentional consecutive blank lines inside it
// is preserved verbatim. Only the join between pieces is normalized.
type seamWriter struct {
	b       strings.Builder
	started bool
}

// write appends s, collapsing the blank run at the seam (this segment's leading
// blanks plus whatever the buffer already ended with) to a single blank line.
func (w *seamWriter) write(s string) {
	if s == "" {
		return
	}
	if w.started {
		// Trailing newlines already in the buffer + leading newlines of s form
		// one run straddling the seam. Cap that combined run: 2+ newlines → one
		// blank line ("\n\n"); exactly one → keep the line break; zero → join
		// with nothing (a mid-line echo splices into the middle of a line and
		// must NOT gain a newline). Only the seam newlines are touched; interior
		// blank lines of either piece are preserved.
		buf := w.b.String()
		trimmedBuf := strings.TrimRight(buf, "\n")
		bufNL := len(buf) - len(trimmedBuf)
		trimmedS := strings.TrimLeft(s, "\n")
		sNL := len(s) - len(trimmedS)

		sep := ""
		switch total := bufNL + sNL; {
		case total >= 2:
			sep = "\n\n"
		case total == 1:
			sep = "\n"
		}
		w.b.Reset()
		w.b.WriteString(trimmedBuf)
		w.b.WriteString(sep)
		w.b.WriteString(trimmedS)
		return
	}
	// First segment: strip its leading blanks so the doc can't open on a blank
	// line, but keep its interior untouched.
	w.b.WriteString(strings.TrimLeft(s, "\n"))
	w.started = true
}

// result returns the assembled output with a single trailing newline.
func (w *seamWriter) result() string {
	return strings.TrimRight(w.b.String(), "\n") + "\n"
}

// Warnings returns the non-fatal issues accumulated during the last Build on
// this runtime (only meaningful in non-strict mode).
func (dr *docRuntime) Warnings() []string { return dr.warnings }

// runStatement executes one statement island. doc.* emit calls append to
// dr.out; we capture what this island produced and reset the buffer for the
// next one.
func (dr *docRuntime) runStatement(seg segment) (string, error) {
	dr.out.Reset()
	dr.pending = nil
	// RunString honors the runtime's WithContext(ctx) deadline internally.
	if err := dr.rt.RunString(seg.body); err != nil {
		return "", dr.wrapLuaErr(seg, err)
	}
	return dr.out.String(), nil
}

// runEcho evaluates an echo island's expression and returns its string value.
// The body is wrapped as `return <expr>` so the runtime yields a value.
func (dr *docRuntime) runEcho(seg segment) (string, error) {
	dr.out.Reset()
	dr.pending = nil
	// RunActionString honors the runtime's WithContext(ctx) deadline internally.
	val, err := dr.rt.RunActionString("return "+seg.body, "manual-echo")
	if err != nil {
		return "", dr.wrapLuaErr(seg, err)
	}
	s, err := coerceEcho(val)
	if err != nil {
		return "", &BuildError{Line: seg.line, Kind: "resolve", Msg: err.Error(), Snip: seg.body}
	}
	// An echo island substitutes inline into a Markdown line, so a trailing
	// newline is never wanted — e.g. description() returns a `|` block scalar
	// that YAML preserves with a trailing "\n", which would otherwise open a
	// double blank against the manual's own blank after the span.
	s = strings.TrimRight(s, "\n")
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
		return strconv.FormatInt(t, 10), nil
	case float64:
		// Whole floats print without a trailing ".0" for tidy counts.
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10), nil
		}
		return strconv.FormatFloat(t, 'g', -1, 64), nil
	case bool:
		return "", errors.New("echo island returned a boolean; inline spans need a string or number")
	default:
		return "", fmt.Errorf("echo island returned %T; inline spans need a string or number (did you mean a fenced rela block?)", v)
	}
}

// wrapLuaErr turns a runtime error into a BuildError anchored to the manual
// line. The Lua frame line is intra-island (1-based within the island body);
// the manual line is the island's start line plus that offset minus one.
func (dr *docRuntime) wrapLuaErr(seg segment, err error) error {
	line := seg.line
	if frames := dr.rt.ErrorFrames(); len(frames) > 0 {
		if fl := frames[len(frames)-1].Line; fl > 0 {
			line = seg.line + fl - 1
		}
	}
	// A doc.* binding that failed stashed a typed BuildError (RaiseError only
	// carries a string, so we can't recover it from err). Prefer it — but ONLY
	// when the raised Lua message still contains the stashed message. If the
	// island caught the resolver's error with pcall and then failed elsewhere,
	// dr.pending is stale and err is the *real* failure: fall through so the
	// author sees that one, not the swallowed one.
	pending := dr.pending
	dr.pending = nil
	if pending != nil && strings.Contains(err.Error(), pending.Msg) {
		pending.Line = line
		pending.Snip = strings.TrimSpace(seg.body)
		return pending
	}
	return &BuildError{Line: line, Kind: "lua", Msg: err.Error(), Snip: strings.TrimSpace(seg.body)}
}

// luaFail records a resolve BuildError and aborts the current island. gopher-lua
// unwinds via RaiseError (string-only), so the typed error is stashed on
// dr.pending and re-attached by wrapLuaErr with the manual line.
func (dr *docRuntime) luaFail(ls *lua.LState, format string, args ...any) int {
	dr.pending = &BuildError{Kind: "resolve", Msg: fmt.Sprintf(format, args...)}
	ls.RaiseError("%s", dr.pending.Msg)
	return 0
}
