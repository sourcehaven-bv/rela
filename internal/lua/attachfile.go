// Lua binding for attaching bytes the script holds (TKT-36DHKH8).
//
//	rela.attach_file{
//	  entity   = "INV-1",            -- the entity that gets the file
//	  property = "pdf",              -- a `file` property of that entity
//	  content  = resp.body,          -- the file's bytes, as a Lua string
//	  filename = "F001.pdf",
//	  mime     = "application/pdf",  -- optional: the type the bytes must be
//	} -> {name, size, sha256, mime} | nil, err
//
// The write runs as the script's principal, through the same gates as an
// upload, and the file is written like an upload of the same bytes.
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

// AttachFileRequest is the parsed argument of rela.attach_file. MIME is ""
// when the script names no type.
type AttachFileRequest struct {
	Entity   string
	Property string
	Content  []byte
	Filename string
	MIME     string
}

// AttachFiler stores a file for rela.attach_file. Defined here at the
// consumer; the data-entry app supplies it. It acts as the principal on ctx,
// and its errors are written for the script author.
type AttachFiler interface {
	AttachFile(ctx context.Context, req AttachFileRequest) (AttachedFile, error)
}

// ErrAttachFileUnavailable is what a runtime without the data-entry app
// answers, so a script shared between runtimes fails clearly.
var ErrAttachFileUnavailable = errors.New("attach_file is not available in this runtime")

// registerAttachFile installs rela.attach_file when the runtime has a filer.
// A method value rather than a closure over r, for the reason
// [registerAttachRender] gives.
func registerAttachFile(r *Runtime, rela *lua.LTable) {
	if r.deps.AttachFile == nil {
		return
	}
	b := attachFileBinding{filer: r.deps.AttachFile}
	r.L.SetField(rela, "attach_file", r.L.NewFunction(b.call))
}

// attachFileBinding is rela.attach_file over one filer.
type attachFileBinding struct {
	filer AttachFiler
}

// attachFileKeys maps each string option to its request field. content is
// read separately because it is bytes.
var attachFileKeys = map[string]func(*AttachFileRequest) *string{
	"entity":   func(q *AttachFileRequest) *string { return &q.Entity },
	"property": func(q *AttachFileRequest) *string { return &q.Property },
	"filename": func(q *AttachFileRequest) *string { return &q.Filename },
	"mime":     func(q *AttachFileRequest) *string { return &q.MIME },
}

func (b attachFileBinding) call(ls *lua.LState) int {
	const binding = "rela.attach_file"
	req, bad := parseAttachFile(ls.CheckTable(1))
	if bad != "" {
		ls.RaiseError("%s: %s", binding, bad)
		return 0
	}
	// The write takes the attachment lock and runs the scan, which must not
	// happen while a store transaction holds its lock.
	ctx := scriptContext(ls)
	if store.InTx(ctx) {
		ls.RaiseError("%s: cannot run inside a store transaction; call it from an action, "+
			"a background automation, a flow step or a scheduled script", binding)
		return 0
	}
	att, err := b.filer.AttachFile(ctx, req)
	return pushAttached(ls, att, err)
}

// parseAttachFile reads the option table. It checks shape only: whether the
// entity, property and bytes are acceptable is the filer's to judge. bad is
// the misuse to raise, or "".
func parseAttachFile(opts *lua.LTable) (req AttachFileRequest, bad string) {
	hasContent := false
	opts.ForEach(func(k, v lua.LValue) {
		if bad != "" {
			return
		}
		key, ok := k.(lua.LString)
		if !ok {
			bad = "options must be a table with string keys"
			return
		}
		field, known := attachFileKeys[string(key)]
		if !known && key != "content" {
			bad = "unknown option " + string(key)
			return
		}
		s, ok := v.(lua.LString)
		if !ok {
			bad = string(key) + " must be a string"
			return
		}
		if key == "content" {
			req.Content = []byte(string(s))
			hasContent = true
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
	case !hasContent:
		return req, "content is required"
	case req.Filename == "":
		return req, "filename is required"
	}
	return req, ""
}
