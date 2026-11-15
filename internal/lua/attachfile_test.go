package lua_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

type fakeAttachFiler struct {
	got  []lua.AttachFileRequest
	tag  any
	err  error
	file lua.AttachedFile
}

func (f *fakeAttachFiler) AttachFile(ctx context.Context, req lua.AttachFileRequest) (lua.AttachedFile, error) {
	f.got = append(f.got, req)
	f.tag = ctx.Value(ctxKey{})
	return f.file, f.err
}

// runAttachFile runs src with f behind rela.attach_file (none when nil).
func runAttachFile(t *testing.T, f lua.AttachFiler, src string) (string, error) {
	t.Helper()
	deps, _ := newHistoryWorld(t)
	deps.AttachFile = f
	ctx := context.WithValue(connectorCtx(), ctxKey{}, "script")
	out, err := runOAuth(ctx, t, deps, lua.Capabilities{}, src)
	return strings.Trim(strings.TrimSpace(out), `"`), err
}

func TestAttachFile_Binding(t *testing.T) {
	f := &fakeAttachFiler{file: lua.AttachedFile{Name: "F001.pdf", Size: 5, SHA256: "abc", MIME: "application/pdf"}}
	// The content carries a NUL and a non-UTF-8 byte: Lua strings are bytes,
	// and the binding must hand them over unchanged.
	out, err := runAttachFile(t, f, `
local att, err = rela.attach_file{entity = "INV-1", property = "pdf",
  content = "%P\0\255F", filename = "F001.pdf", mime = "application/pdf"}
rela.output(att.name .. " " .. att.size .. " " .. att.sha256 .. " " .. att.mime .. " " .. tostring(err))`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "F001.pdf 5 abc application/pdf nil" {
		t.Errorf("output %q", out)
	}
	if len(f.got) != 1 {
		t.Fatalf("calls = %d, want 1", len(f.got))
	}
	got := f.got[0]
	if got.Entity != "INV-1" || got.Property != "pdf" || got.Filename != "F001.pdf" || got.MIME != "application/pdf" {
		t.Errorf("request %+v", got)
	}
	if string(got.Content) != "%P\x00\xffF" {
		t.Errorf("content %q", got.Content)
	}
	if f.tag != "script" {
		t.Error("the filer did not get the script's context")
	}
}

func TestAttachFile_MIMEAndEmptyContentAreOptional(t *testing.T) {
	f := &fakeAttachFiler{}
	if _, err := runAttachFile(t, f, `rela.attach_file{entity = "A", property = "p", content = "", filename = "x"}`); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 1 || f.got[0].MIME != "" || len(f.got[0].Content) != 0 {
		t.Errorf("request %+v", f.got)
	}
}

func TestAttachFile_ErrorIsReturned(t *testing.T) {
	f := &fakeAttachFiler{err: errors.New("attach_file: permission denied")}
	out, err := runAttachFile(t, f, `
local att, err = rela.attach_file{entity = "A", property = "p", content = "x", filename = "x.txt"}
rela.output(tostring(att) .. " " .. err)`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "nil attach_file: permission denied" {
		t.Errorf("output %q", out)
	}
}

func TestAttachFile_AbsentWithoutFiler(t *testing.T) {
	out, err := runAttachFile(t, nil, `rela.output(tostring(rela.attach_file))`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "nil" {
		t.Errorf("binding present without a filer: %q", out)
	}
}

func TestAttachFile_RefusedInTx(t *testing.T) {
	deps, _ := newHistoryWorld(t)
	f := &fakeAttachFiler{}
	deps.AttachFile = f
	ctx := store.ContextInTx(connectorCtx())
	_, err := runOAuth(ctx, t, deps, lua.Capabilities{},
		`rela.attach_file{entity = "A", property = "p", content = "x", filename = "x.txt"}`)
	if err == nil || !strings.Contains(err.Error(), "store transaction") {
		t.Fatalf("err = %v, want a transaction refusal", err)
	}
	if len(f.got) != 0 {
		t.Error("the filer ran inside a transaction")
	}
}

func TestAttachFile_ArgumentMisuseRaises(t *testing.T) {
	const ok = `entity = "A", property = "p", content = "x", filename = "x.txt"`
	for _, tc := range []struct{ name, args, want string }{
		{"no table", `"A"`, "table expected"},
		{"positional value", `{"A"}`, "string keys"},
		{"unknown option", `{` + ok + `, face = "x"}`, "unknown option face"},
		{"unknown non-string option", `{` + ok + `, face = 3}`, "unknown option face"},
		{"non-string content", `{entity = "A", property = "p", content = {}, filename = "x"}`, "content must be a string"},
		{"non-string mime", `{` + ok + `, mime = 3}`, "mime must be a string"},
		{"no entity", `{property = "p", content = "x", filename = "x"}`, "entity is required"},
		{"no property", `{entity = "A", content = "x", filename = "x"}`, "property is required"},
		{"no content", `{entity = "A", property = "p", filename = "x"}`, "content is required"},
		{"no filename", `{entity = "A", property = "p", content = "x"}`, "filename is required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAttachFiler{}
			_, err := runAttachFile(t, f, `rela.attach_file(`+tc.args+`)`)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
			if len(f.got) != 0 {
				t.Error("the filer ran on a bad call")
			}
		})
	}
}
