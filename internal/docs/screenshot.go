package docs

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// defaultCropPad is the padding (CSS px) added around a clipped region when the
// author does not specify pad=. Matches openvwr's figure convention.
const defaultCropPad = 24

// Capturer renders a screenshot{} island: it stands up the data-entry SPA over a
// seeded temp project, drives a headless browser to the requested view, and
// writes a PNG. It is a consumer-side interface (defined here, in the consumer)
// so the core docs package never imports the browser/data-entry machinery — the
// concrete implementation (internal/docscapture) is injected by the CLI. A nil
// Capturer means screenshot{} fails loud.
//
// Capture returns the absolute path of the written PNG.
type Capturer interface {
	Capture(ctx context.Context, spec CaptureSpec) (pngPath string, err error)
	// Close tears down any temp project / server / browser the Capturer stood up.
	Close() error
}

// CaptureSpec is the request for one screenshot. It is a plain DTO so the
// interface has no dependency on the doc runtime's internals.
type CaptureSpec struct {
	// ProjectDir is the documented project (schema/config copied into the temp
	// project the SPA renders).
	ProjectDir string
	// Seed is replayed against the temp project's store so the SPA renders the
	// same entities the manual created.
	Seed []SeedOp

	// View is the SPA view kind: "form" (edit form), "entity" (detail), "list".
	View string
	// Type is the entity type; Entity is the entity id to render.
	Type   string
	Entity string
	// Form is the data-entry.yaml form id for View=="form"/"create" (derived
	// from Type when empty: edit_<type> / new_<type>).
	Form string
	// List is the data-entry.yaml list id for View=="list".
	List string
	// Query is the search term for View=="search".
	Query string
	// World is the `?world=` the page is opened in. Empty ⇒ the app's
	// configured default_world, i.e. what a user gets by just navigating.
	//
	// Screenshots are how a reader SEES that a world changes what a page
	// shows, so a manual comparing two worlds needs to open the same page
	// twice. Validated against the declared worlds before capture, because a
	// typo would silently render the default and the figure would illustrate
	// the opposite of its caption.
	World string
	// As is the role to render as (mapped to a principal assigned that role);
	// empty ⇒ the harness picks a default role that can read.
	As string

	// Clip bounds the capture. Empty ⇒ the full page. A CSS selector
	// ("#field-status", ".form-section") ⇒ that element. A predefined keyword ⇒
	// a computed region: "focus" ⇒ the bounding box of all annotated targets.
	// The clipped region is expanded by Pad and clamped to the page.
	Clip string
	// Pad is the padding in CSS px added around a Clip region (default via the
	// resolver). Ignored for a full-page capture.
	Pad int
	// Annotations to draw before capture.
	Arrows []Annotation

	// AwaitVersions is how many version rows View=="history" must show before
	// the capture is taken. Zero ⇒ capture whatever is on screen.
	//
	// # Why a wait is needed at all, and why it is a COUNT
	//
	// On the postgres backend create/update versions are captured by a
	// DEBOUNCED reconciliation sweep, not synchronously with the write (see the
	// pgstore sweep). So a history page opened immediately after an edit
	// legitimately shows an empty timeline: the version exists in the future,
	// not in the database. A capture taken then photographs "No versions
	// recorded yet" under a caption promising a history — the manual would be
	// lying, and lying intermittently, which is worse than failing.
	//
	// Waiting on the RENDERED ROW COUNT rather than on a duration is what makes
	// this deterministic. A sleep encodes a guess about sweep cadence that is
	// wrong on a slow machine and wasteful on a fast one; a count is the actual
	// condition the figure depends on. It is also self-verifying: a manual that
	// claims three versions and gets two FAILS the build rather than quietly
	// publishing a figure that contradicts its own prose.
	AwaitVersions int

	// OutPath is the absolute PNG path to write.
	OutPath string
}

// Annotation kinds.
const (
	// AnnotationArrow draws an arrow (with optional Text label) into the target.
	AnnotationArrow = "arrow"
	// AnnotationBox draws an outline box around the target.
	AnnotationBox = "box"
)

// Annotation is one mark drawn on the capture, anchored to a form field
// (At="<property>" → #field-<property>) or an ARIA/control target
// (At="@button:save" / "@role:..."). Kind selects the mark ("arrow" default,
// "box"); Text rides an arrow. Kind is a string (not a bool) so new mark types
// can be added without a breaking DTO change.
type Annotation struct {
	At   string
	Text string
	Side string // "left"|"right"|"top"|"bottom"; default right
	Kind string // "arrow" (default) | "box"
}

// Screenshots share the document's ONE temp project with api{} and page{}, and
// islands run top-to-bottom, so a figure renders whatever earlier islands
// wrote — including a real write issued through api{}. See the ordering notes
// on [docRuntime.luaAPI] for what that does and does not guarantee.
//
// luaScreenshot is the screenshot{} island resolver. It emits a Markdown image
// reference and writes the PNG next to the manual output via the injected
// Capturer. Fails loud when no Capturer is wired (browser support absent).
//
// screenshot{ view="form", type="ticket", entity=id, as="editor",
//
//	arrows={{at="status", text="auto-computed"}}, out="fig.png", alt="..." }
func (b *tierBBindings) luaScreenshot(ls *lua.LState) int {
	tbl := argTable(ls)
	if tbl == nil {
		return b.luaFail(ls, "screenshot: expects a table, e.g. screenshot{view=\"form\", type=..., entity=...}")
	}
	if b.capturer == nil {
		reason := b.capturerErr
		if reason == "" {
			reason = "screenshot{} needs a Chrome/Chromium browser and a built data-entry SPA"
		}
		return b.luaFail(ls, "screenshot: no browser capturer available — %s", reason)
	}

	if w := fieldString(ls, tbl, "world"); w != "" {
		if err := b.validateWorld(w); err != nil {
			return b.luaFail(ls, "screenshot: %v", err)
		}
	}

	spec := CaptureSpec{
		ProjectDir: b.projectDir,
		Seed:       b.seed(),
		View:       fieldStringDefault(ls, tbl, "view", "form"),
		Type:       fieldString(ls, tbl, "type"),
		Entity:     fieldString(ls, tbl, "entity"),
		Form:       fieldString(ls, tbl, "form"),
		As:         fieldString(ls, tbl, "as"),
		World:      fieldString(ls, tbl, "world"),
		List:       fieldString(ls, tbl, "list"),
		Query:      fieldString(ls, tbl, "q"),
		Clip:       fieldString(ls, tbl, "clip"),
		Pad:        fieldInt(ls, tbl, "pad", defaultCropPad),
		Arrows:     b.readAnnotations(ls, tbl),

		AwaitVersions: fieldInt(ls, tbl, "await_versions", 0),
	}
	// Every view the SPA routes and stamps a readiness marker on. A view with
	// no marker has nothing for the capture to poll and could only hang until
	// the timeout, so unknown ones are rejected loudly.
	if !supportedView(spec.View) {
		return b.luaFail(ls, "screenshot: unknown view %q — one of %s",
			spec.View, strings.Join(supportedViews, ", "))
	}
	// Each view needs different arguments, and asking for the wrong one is the
	// difference between a clear refusal and a capture of the wrong screen. The
	// rules are shared with page{} so the two verbs cannot disagree about what
	// names a screen.
	if err := requireViewArgs(spec); err != nil {
		return b.luaFail(ls, "screenshot: %v", err)
	}

	// `await_versions` waits on the history timeline's rendered rows, which only
	// the history view has. Silently ignoring it elsewhere would let an author
	// believe a capture waits for something when it does not.
	if spec.AwaitVersions != 0 && spec.View != "history" {
		return b.luaFail(ls, "screenshot{view=%q}: `await_versions` applies only to "+
			"view=\"history\" — no other screen has a version timeline to wait for", spec.View)
	}
	if spec.AwaitVersions < 0 {
		return b.luaFail(ls, "screenshot{await_versions=%d}: must be positive", spec.AwaitVersions)
	}

	out := fieldString(ls, tbl, "out")
	if out == "" {
		return b.luaFail(ls, "screenshot: `out` is required (the image file to write, e.g. out=\"fig.png\")")
	}
	spec.OutPath = b.resolveOutPath(out)

	png, err := b.capturer.Capture(b.ctx, spec)
	if err != nil {
		return b.luaFail(ls, "screenshot(%s %q): %v", spec.View, spec.Entity, err)
	}

	alt := fieldStringDefault(ls, tbl, "alt", fmt.Sprintf("%s %s", spec.View, spec.Type))
	// Emit a relative path from the output dir so the markdown is portable.
	rel := b.relOutPath(png)
	b.emit(fmt.Sprintf("![%s](%s)\n\n", mdCell(alt), rel))
	return 0
}

// readAnnotations parses the optional arrows={{at=..,text=..,side=..},{...}} arg.
func (b *tierBBindings) readAnnotations(ls *lua.LState, tbl *lua.LTable) []Annotation {
	arr, ok := ls.GetField(tbl, "arrows").(*lua.LTable)
	if !ok {
		return nil
	}
	var out []Annotation
	arr.ForEach(func(_, v lua.LValue) {
		a, ok := v.(*lua.LTable)
		if !ok {
			return
		}
		kind := fieldString(ls, a, "kind")
		if kind == "" && fieldBool(ls, a, "box") {
			kind = AnnotationBox // back-compat: box=true still works
		}
		if kind == "" {
			kind = AnnotationArrow
		}
		out = append(out, Annotation{
			At:   fieldString(ls, a, "at"),
			Text: fieldString(ls, a, "text"),
			Side: fieldString(ls, a, "side"),
			Kind: kind,
		})
	})
	return out
}

// resolveOutPath makes an operator-supplied `out` absolute, relative to the
// build's output directory (outDir). A traversal outside outDir is rejected by
// the caller path handling; here we just join.
func (b *tierBBindings) resolveOutPath(out string) string {
	if filepath.IsAbs(out) {
		return out
	}
	if b.outDir != "" {
		return filepath.Join(b.outDir, out)
	}
	return out
}

// relOutPath returns the PNG path relative to the output dir for the markdown
// image reference, so the emitted manual is portable next to its images.
func (b *tierBBindings) relOutPath(png string) string {
	if b.outDir == "" {
		return filepath.Base(png)
	}
	if rel, err := filepath.Rel(b.outDir, png); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return filepath.Base(png)
}

// supportedViews are the screen kinds a capture can wait for: each routes in
// the SPA and stamps a readiness marker (`form-state-*` or `page-state-*`) the
// renderability gate polls. A view without one could only hang until the
// capture timeout, which is why this is an allowlist rather than a passthrough.
var supportedViews = []string{
	"analyze", "calendar", "create", "dashboard", "entity",
	"form", "history", "kanban", "list", "search",
}

func supportedView(v string) bool { return slices.Contains(supportedViews, v) }
