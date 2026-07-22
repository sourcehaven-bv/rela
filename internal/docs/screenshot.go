package docs

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

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
	// Form is the data-entry.yaml form id for View=="form" (default derived).
	Form string
	// As is the role to render as (mapped to a principal assigned that role);
	// empty ⇒ the harness picks a default role that can read.
	As string

	// Clip is an optional CSS selector to clip the capture to one element.
	Clip string
	// Annotations to draw before capture.
	Arrows []Annotation

	// OutPath is the absolute PNG path to write.
	OutPath string
}

// Annotation is one arrow/box drawn on the capture, anchored to a form field
// (At="<property>" → #field-<property>) or an ARIA/control target
// (At="@button:save" / "@role:..."). Text rides the arrow.
type Annotation struct {
	At   string
	Text string
	Side string // "left"|"right"|"top"|"bottom"; default right
	Box  bool   // draw an outline box instead of an arrow
}

// luaScreenshot is the screenshot{} island resolver. It emits a Markdown image
// reference and writes the PNG next to the manual output via the injected
// Capturer. Fails loud when no Capturer is wired (browser support absent).
//
// screenshot{ view="form", type="ticket", entity=id, as="editor",
//
//	arrows={{at="status", text="auto-computed"}}, out="fig.png", alt="..." }
func (dr *docRuntime) luaScreenshot(ls *lua.LState) int {
	tbl := argTable(ls)
	if tbl == nil {
		return dr.luaFail(ls, "screenshot: expects a table, e.g. screenshot{view=\"form\", type=..., entity=...}")
	}
	if dr.capturer == nil {
		return dr.luaFail(ls, "screenshot: no browser capturer configured — screenshot{} needs a Chrome/Chromium browser and a built data-entry SPA")
	}

	spec := CaptureSpec{
		ProjectDir: dr.projectDir,
		Seed:       dr.seedOps,
		View:       fieldStringDefault(ls, tbl, "view", "form"),
		Type:       fieldString(ls, tbl, "type"),
		Entity:     fieldString(ls, tbl, "entity"),
		Form:       fieldString(ls, tbl, "form"),
		As:         fieldString(ls, tbl, "as"),
		Clip:       fieldString(ls, tbl, "clip"),
		Arrows:     dr.readAnnotations(ls, tbl),
	}
	if spec.Type == "" {
		return dr.luaFail(ls, "screenshot: `type` is required")
	}
	if spec.Entity == "" {
		return dr.luaFail(ls, "screenshot: `entity` is required (the id of a seeded entity to render)")
	}

	out := fieldString(ls, tbl, "out")
	if out == "" {
		return dr.luaFail(ls, "screenshot: `out` is required (the image file to write, e.g. out=\"fig.png\")")
	}
	spec.OutPath = dr.resolveOutPath(out)

	png, err := dr.capturer.Capture(dr.ctx, spec)
	if err != nil {
		return dr.luaFail(ls, "screenshot(%s %q): %v", spec.View, spec.Entity, err)
	}

	alt := fieldStringDefault(ls, tbl, "alt", fmt.Sprintf("%s %s", spec.View, spec.Type))
	// Emit a relative path from the output dir so the markdown is portable.
	rel := dr.relOutPath(png)
	dr.emit(fmt.Sprintf("![%s](%s)\n\n", mdCell(alt), rel))
	return 0
}

// readAnnotations parses the optional arrows={{at=..,text=..,side=..},{...}} arg.
func (dr *docRuntime) readAnnotations(ls *lua.LState, tbl *lua.LTable) []Annotation {
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
		out = append(out, Annotation{
			At:   fieldString(ls, a, "at"),
			Text: fieldString(ls, a, "text"),
			Side: fieldString(ls, a, "side"),
			Box:  fieldBool(ls, a, "box"),
		})
	})
	return out
}

// resolveOutPath makes an operator-supplied `out` absolute, relative to the
// build's output directory (outDir). A traversal outside outDir is rejected by
// the caller path handling; here we just join.
func (dr *docRuntime) resolveOutPath(out string) string {
	if filepath.IsAbs(out) {
		return out
	}
	if dr.outDir != "" {
		return filepath.Join(dr.outDir, out)
	}
	return out
}

// relOutPath returns the PNG path relative to the output dir for the markdown
// image reference, so the emitted manual is portable next to its images.
func (dr *docRuntime) relOutPath(png string) string {
	if dr.outDir == "" {
		return filepath.Base(png)
	}
	if rel, err := filepath.Rel(dr.outDir, png); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return filepath.Base(png)
}
