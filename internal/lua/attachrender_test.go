package lua_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

type fakeAttachRenderer struct {
	got  []lua.AttachRenderRequest
	tag  any
	err  error
	file lua.AttachedFile
}

func (f *fakeAttachRenderer) AttachRender(ctx context.Context, req lua.AttachRenderRequest) (lua.AttachedFile, error) {
	f.got = append(f.got, req)
	f.tag = ctx.Value(ctxKey{})
	return f.file, f.err
}

type ctxKey struct{}

// runAttach runs src with r behind rela.attach_render (none when nil).
func runAttach(t *testing.T, r lua.AttachRenderer, src string) (string, error) {
	t.Helper()
	deps, _ := newHistoryWorld(t)
	deps.AttachRender = r
	ctx := context.WithValue(connectorCtx(), ctxKey{}, "script")
	out, err := runOAuth(ctx, t, deps, lua.Capabilities{}, src)
	// rela.output writes a JSON value.
	if unquoted, uerr := strconv.Unquote(strings.TrimSpace(out)); uerr == nil {
		out = unquoted
	}
	return out, err
}

func TestAttachRender_Binding(t *testing.T) {
	f := &fakeAttachRenderer{file: lua.AttachedFile{Name: "r.pdf", Size: 42, SHA256: "abc", MIME: "application/pdf"}}
	out, err := runAttach(t, f, `
local att, err = rela.attach_render{entity = "AUD-1", property = "report",
  document = "audit", entry = "AUD-1", format = "pdf", filename = "r.pdf"}
rela.output(att.name .. " " .. att.size .. " " .. att.sha256 .. " " .. att.mime .. " " .. tostring(err))`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "r.pdf 42 abc application/pdf nil" {
		t.Errorf("output %q", out)
	}
	want := lua.AttachRenderRequest{
		Entity: "AUD-1", Property: "report", Document: "audit", Entry: "AUD-1", Format: "pdf", Filename: "r.pdf",
	}
	if len(f.got) != 1 || f.got[0] != want {
		t.Errorf("request %+v, want %+v", f.got, want)
	}
	if f.tag != "script" {
		t.Error("the renderer did not get the script's context")
	}
}

func TestAttachRender_ErrorIsReturned(t *testing.T) {
	f := &fakeAttachRenderer{err: errors.New("attach_render: permission denied")}
	out, err := runAttach(t, f, `
local att, err = rela.attach_render{entity = "A", property = "p", list = "l", format = "md", filename = "x.md"}
rela.output(tostring(att) .. " " .. err)`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "nil attach_render: permission denied" {
		t.Errorf("output %q", out)
	}
}

func TestAttachRender_AbsentWithoutRenderer(t *testing.T) {
	out, err := runAttach(t, nil, `rela.output(tostring(rela.attach_render))`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "nil" {
		t.Errorf("binding present without a renderer: %q", out)
	}
}

func TestAttachRender_RefusedInTx(t *testing.T) {
	deps, _ := newHistoryWorld(t)
	f := &fakeAttachRenderer{}
	deps.AttachRender = f
	ctx := store.ContextInTx(connectorCtx())
	_, err := runOAuth(ctx, t, deps, lua.Capabilities{}, `rela.attach_render{entity = "A", property = "p",
  list = "l", format = "md", filename = "x.md"}`)
	if err == nil || !strings.Contains(err.Error(), "store transaction") {
		t.Fatalf("err = %v, want a transaction refusal", err)
	}
	if len(f.got) != 0 {
		t.Error("the renderer ran inside a transaction")
	}
}

func TestAttachRender_ArgumentMisuseRaises(t *testing.T) {
	const ok = `entity = "A", property = "p", format = "md", filename = "x.md"`
	for _, tc := range []struct{ name, args, want string }{
		{"no table", `"A"`, "table expected"},
		{"positional value", `{"A"}`, "string keys"},
		{"unknown option", `{` + ok + `, list = "l", face = "x"}`, "unknown option face"},
		{"non-string value", `{` + ok + `, list = 3}`, "list must be a string"},
		{"no entity", `{property = "p", list = "l", format = "md", filename = "x"}`, "entity is required"},
		{"no property", `{entity = "A", list = "l", format = "md", filename = "x"}`, "property is required"},
		{"neither source", `{` + ok + `}`, "exactly one of document and list"},
		{"both sources", `{` + ok + `, list = "l", document = "d"}`, "exactly one of document and list"},
		{"entry with list", `{` + ok + `, list = "l", entry = "A"}`, "entry applies to a document"},
		{"no format", `{entity = "A", property = "p", list = "l", filename = "x"}`, "format is required"},
		{"no filename", `{entity = "A", property = "p", list = "l", format = "md"}`, "filename is required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAttachRenderer{}
			_, err := runAttach(t, f, `rela.attach_render(`+tc.args+`)`)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
			if len(f.got) != 0 {
				t.Error("the renderer ran on a bad call")
			}
		})
	}
}
