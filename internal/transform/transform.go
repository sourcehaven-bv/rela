// Package transform is the view-export engine: it converts markdown produced by
// a [Renderer] into an output format (PDF, DOCX, ...) by running a named,
// project-configured external command.
//
// The design is a cross product, not an N×M table: register a format once (in
// the metamodel `transforms:` map) and every surface that can produce markdown
// — an entity view, a list view, a Lua document — gains that format for free.
//
// Two layers meet here:
//
//   - A [Registry] of named [Def]s. Each Def is a `from: markdown` → format byte
//     shuttle: an argv command template (with {in}/{out} placeholders) plus the
//     produced content-type. Defs know nothing about entities, Lua, or the web
//     app — pure byte conversion, executed via [github.com/Sourcehaven-BV/rela/internal/cmdexec].
//   - A [Renderer], defined at this call site, that produces the markdown. The
//     built-in [EntityRenderer] lives here; ACL-coupled renderers (e.g. a list
//     table) are supplied by the caller (data-entry) as a Renderer implementation.
//
// Safety: transform commands come from project config (metamodel.yaml), the same
// trust level as scan/transform attachment commands and schedules — NOT from a
// request. A request may only choose a registered format *name*; it never
// supplies a command, flag, or path. Execution is argv-array (no shell), with a
// timeout and an output-size cap (see cmdexec).
package transform

import (
	"cmp"
	"context"
	"fmt"
	"slices"
)

// FormatMarkdown is the only input format supported in v1. A Def whose From is
// not this is rejected at metamodel load.
const FormatMarkdown = "markdown"

// Def is one registered transform: an external command converting From-format
// bytes to the Produces content-type.
type Def struct {
	// From is the input format the command consumes. v1: always "markdown".
	From string
	// Command is the argv array run per export. It may reference the
	// [github.com/Sourcehaven-BV/rela/internal/cmdexec] {in}/{out} placeholders;
	// otherwise input is fed on stdin and output read from stdout.
	Command []string
	// Produces is the output content-type (e.g. "application/pdf"), echoed into
	// the export response Content-Type. Validated as a well-formed media type at
	// metamodel load.
	Produces string
}

// Registry maps a transform name (as used in `--transform <name>` / the export
// menu) to its [Def].
type Registry map[string]Def

// NamedDef pairs a transform's name with its Def for listing (e.g. the export
// menu / GET /api/transforms).
type NamedDef struct {
	Name string
	Def  Def
}

// FromMarkdown returns the transforms whose input is markdown, in name order.
// This is exactly the set of formats every markdown [Renderer] can export to.
func (r Registry) FromMarkdown() []NamedDef {
	out := make([]NamedDef, 0, len(r))
	for name, def := range r {
		if def.From == FormatMarkdown {
			out = append(out, NamedDef{Name: name, Def: def})
		}
	}
	slices.SortFunc(out, func(a, b NamedDef) int { return cmp.Compare(a.Name, b.Name) })
	return out
}

// Renderer produces the markdown that a transform converts. It is a call-site
// interface: implementations range from the built-in [EntityRenderer] to an
// ACL-scoped list renderer supplied by data-entry.
type Renderer interface {
	// Render returns the markdown bytes for the subject being exported. It runs
	// against an already-authorized snapshot; a Renderer performs no ACL decision
	// of its own.
	Render(ctx context.Context) ([]byte, error)
}

// RendererFunc adapts a plain function to [Renderer].
type RendererFunc func(ctx context.Context) ([]byte, error)

// Render calls f.
func (f RendererFunc) Render(ctx context.Context) ([]byte, error) { return f(ctx) }

// UnknownTransformError is returned when an export names a transform absent from
// the registry. Callers should map it to a 4xx, not a 500 — it is caller input.
type UnknownTransformError struct{ Name string }

func (e UnknownTransformError) Error() string {
	return fmt.Sprintf("transform: unknown transform %q", e.Name)
}
