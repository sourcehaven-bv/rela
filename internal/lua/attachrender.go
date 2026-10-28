// Lua binding for attaching a rendered document or list (TKT-1TS55R).
//
//	rela.attach_render{
//	  entity   = "AUD-1",          -- the entity that gets the file
//	  property = "report",         -- a `file` property of that entity
//	  document = "audit-report",   -- a configured document, or
//	  list     = "open-findings",  -- a configured list (exactly one of the two)
//	  entry    = "AUD-1",          -- the entry entity of an entity-anchored document
//	  format   = "pdf",            -- a registered transform, or "md"
//	  filename = "report.pdf",
//	} -> {name, size, sha256, mime} | nil, err
//
// The render runs as the script's principal, through the same gates as the
// export routes, and the file is written like an upload of the same bytes.
// Refusals and failures return nil and a message; only a misuse of the
// arguments raises.
//
// A free function rather than a Runtime method: Runtime is at its plimsoll
// method cap.
package lua

import (
	"context"
	"errors"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// AttachRenderRequest is the parsed argument of rela.attach_render. Exactly
// one of Document and List is set; Entry only with Document.
type AttachRenderRequest struct {
	Entity   string
	Property string
	Document string
	List     string
	Entry    string
	Format   string
	Filename string
}

// AttachedFile describes the stored file. SHA256 is the lowercase hex
// SHA-256 of the stored bytes.
type AttachedFile struct {
	Name   string
	Size   int64
	SHA256 string
	MIME   string
}

// AttachRenderer renders and attaches for rela.attach_render. Defined here at
// the consumer; the data-entry app supplies it. It acts as the principal on
// ctx, and its errors are written for the script author.
type AttachRenderer interface {
	AttachRender(ctx context.Context, req AttachRenderRequest) (AttachedFile, error)
}

// ErrAttachRenderUnavailable is what a runtime without the data-entry app
// answers, so a script shared between runtimes fails clearly.
var ErrAttachRenderUnavailable = errors.New("attach_render is not available in this runtime")

// registerAttachRender installs rela.attach_render when the runtime has a
// renderer.
//
// The binding is a method value rather than a closure over r: contextcheck
// follows a registration closure back to every runtime constructor and asks
// for a context there (see the Runtime plimsoll note on mail.send).
func registerAttachRender(r *Runtime, rela *lua.LTable) {
	if r.deps.AttachRender == nil {
		return
	}
	b := attachRenderBinding{renderer: r.deps.AttachRender}
	r.L.SetField(rela, "attach_render", r.L.NewFunction(b.call))
}

// attachRenderBinding is rela.attach_render over one renderer.
type attachRenderBinding struct {
	renderer AttachRenderer
}

// attachRenderKeys maps each option to its request field.
var attachRenderKeys = map[string]func(*AttachRenderRequest) *string{
	"entity":   func(q *AttachRenderRequest) *string { return &q.Entity },
	"property": func(q *AttachRenderRequest) *string { return &q.Property },
	"document": func(q *AttachRenderRequest) *string { return &q.Document },
	"list":     func(q *AttachRenderRequest) *string { return &q.List },
	"entry":    func(q *AttachRenderRequest) *string { return &q.Entry },
	"format":   func(q *AttachRenderRequest) *string { return &q.Format },
	"filename": func(q *AttachRenderRequest) *string { return &q.Filename },
}

func (b attachRenderBinding) call(ls *lua.LState) int {
	const binding = "rela.attach_render"
	req, bad := parseAttachRender(ls.CheckTable(1))
	if bad != "" {
		ls.RaiseError("%s: %s", binding, bad)
		return 0
	}
	// A render and a conversion are slow external work, which must not run
	// while a store transaction holds its lock. The script context derives
	// from the caller's, so it carries the transaction mark.
	ctx := scriptContext(ls)
	if store.InTx(ctx) {
		ls.RaiseError("%s: cannot run inside a store transaction; call it from an action, "+
			"a background automation or a scheduled script", binding)
		return 0
	}
	att, err := b.renderer.AttachRender(ctx, req)
	if err != nil {
		ls.Push(lua.LNil)
		ls.Push(lua.LString(err.Error()))
		return 2
	}
	t := ls.NewTable()
	t.RawSetString("name", lua.LString(att.Name))
	t.RawSetString("size", lua.LNumber(att.Size))
	t.RawSetString("sha256", lua.LString(att.SHA256))
	t.RawSetString("mime", lua.LString(att.MIME))
	ls.Push(t)
	return 1
}

// scriptContext is the context the runtime runs the script under. It carries
// the script deadline and the caller's principal, so a render that outlives
// the script is cancelled with it. A runtime with neither has no context.
func scriptContext(ls *lua.LState) context.Context {
	if ctx := ls.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

// parseAttachRender reads the option table. It checks shape only: which
// names are configured is the renderer's to judge. bad is the misuse to
// raise, or "".
func parseAttachRender(opts *lua.LTable) (req AttachRenderRequest, bad string) {
	opts.ForEach(func(k, v lua.LValue) {
		if bad != "" {
			return
		}
		key, ok := k.(lua.LString)
		if !ok {
			bad = "options must be a table with string keys"
			return
		}
		field, known := attachRenderKeys[string(key)]
		if !known {
			bad = "unknown option " + string(key)
			return
		}
		s, ok := v.(lua.LString)
		if !ok {
			bad = string(key) + " must be a string"
			return
		}
		*field(&req) = string(s)
	})
	if bad != "" {
		return req, bad
	}
	switch {
	case req.Entity == "":
		return req, "entity is required"
	case req.Property == "":
		return req, "property is required"
	case (req.Document == "") == (req.List == ""):
		return req, "set exactly one of document and list"
	case req.List != "" && req.Entry != "":
		return req, "entry applies to a document, not a list"
	case req.Format == "":
		return req, "format is required"
	case req.Filename == "":
		return req, "filename is required"
	}
	return req, ""
}
