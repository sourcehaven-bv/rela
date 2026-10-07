package lua_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// recordingTagWriter records the tag writes a script makes.
type recordingTagWriter struct {
	calls []string
	err   error
}

func (w *recordingTagWriter) TagCurrent(
	ctx context.Context, ref entity.Ref, name store.VersionTagName, expect store.EntityVersion,
	view func(context.Context, entity.Ref) (*entity.Entity, error),
) (store.VersionMeta, error) {
	call := "current " + ref.String() + " " + name.String() + " " + string(expect)
	if expect != "" {
		// The binding must hand over the script's own reader, which the
		// manager reads the token's entity through.
		if e, err := view(ctx, ref); err == nil {
			call += " seen=" + e.ID
		}
	}
	w.calls = append(w.calls, call)
	return store.VersionMeta{Version: 7}, w.err
}

func (w *recordingTagWriter) TagVersion(
	_ context.Context, ref entity.Ref, name store.VersionTagName, version int,
) (store.VersionMeta, error) {
	w.calls = append(w.calls, "version "+ref.String()+" "+name.String())
	return store.VersionMeta{Version: version}, w.err
}

func (w *recordingTagWriter) UntagVersion(_ context.Context, ref entity.Ref, name store.VersionTagName) error {
	w.calls = append(w.calls, "untag "+ref.String()+" "+name.String())
	return w.err
}

func TestVersionTags_WriteBindings(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		err       error
		wantOut   string
		wantCalls string
		wantErr   string
	}{
		{
			name:    "current, unconditional",
			src:     `rela.output("v=" .. rela.tag_version("P-1", "reviewed"))`,
			wantOut: "v=7", wantCalls: "current P-1 reviewed ",
		},
		{
			name:    "current, expect",
			src:     `rela.output("v=" .. rela.tag_version("P-1", "sync/jira", {expect = "tok"}))`,
			wantOut: "v=7", wantCalls: "current P-1 sync/jira tok seen=P-1",
		},
		{
			name:    "version",
			src:     `rela.output("v=" .. rela.tag_version("P-1", "reviewed", {version = 2}))`,
			wantOut: "v=2", wantCalls: "version P-1 reviewed",
		},
		{
			name:    "conflict returns nil and the reason",
			src:     `local v, why = rela.tag_version("P-1", "reviewed", {expect = "old"}); rela.output(tostring(v) .. "," .. why)`,
			err:     &store.VersionConflictError{},
			wantOut: "nil,conflict", wantCalls: "current P-1 reviewed old seen=P-1",
		},
		{
			name:    "untag",
			src:     `rela.output("u=" .. tostring(rela.untag_version("P-1", "reviewed")))`,
			wantOut: "u=true", wantCalls: "untag P-1 reviewed",
		},
		{
			name:    "untag of an absent tag",
			src:     `rela.output("u=" .. tostring(rela.untag_version("P-1", "reviewed")))`,
			err:     store.ErrNotFound,
			wantOut: "u=false", wantCalls: "untag P-1 reviewed",
		},
		{
			name:    "store refusal raises",
			src:     `rela.tag_version("P-1", "reviewed")`,
			err:     store.ErrTagInTx,
			wantErr: store.ErrTagInTx.Error(), wantCalls: "current P-1 reviewed ",
		},
		{name: "bad name", src: `rela.tag_version("P-1", "Bad Name")`, wantErr: "rela.tag_version"},
		{name: "expect and version", src: `rela.tag_version("P-1", "x", {expect = "t", version = 1})`, wantErr: "not both"},
		{name: "bad version", src: `rela.tag_version("P-1", "x", {version = 1.5})`, wantErr: "positive integer"},
		{name: "empty expect", src: `rela.tag_version("P-1", "x", {expect = ""})`, wantErr: "non-empty"},
		{name: "hidden entity", src: `rela.tag_version("SEC-1", "x")`, wantErr: "entity not found: SEC-1"},
		{name: "missing entity", src: `rela.untag_version("NOPE-1", "x")`, wantErr: "entity not found: NOPE-1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := newHistoryWorld(t)
			w := &recordingTagWriter{err: tc.err}
			deps.VersionTags = w
			var out string
			var err error
			if tc.wantErr == "" {
				out = runAsAlice(t, deps, tc.src)
			} else {
				err = runAsAliceErr(deps, tc.src)
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
				}
			}
			if !strings.Contains(out, tc.wantOut) {
				t.Errorf("output %q lacks %q", out, tc.wantOut)
			}
			if got := strings.Join(w.calls, ";"); got != tc.wantCalls {
				t.Errorf("calls = %q, want %q", got, tc.wantCalls)
			}
		})
	}
}

func TestVersionTags_WriteBindingsAbsentWithoutWriter(t *testing.T) {
	deps := newHistoryWorld(t)
	out := runAsAlice(t, deps, `rela.output(type(rela.tag_version) .. "," .. type(rela.untag_version))`)
	if !strings.Contains(out, "nil,nil") {
		t.Errorf("output %q: the write bindings exist without a writer", out)
	}
}

func TestVersionTags_TokenAndHistoryTags(t *testing.T) {
	deps := newHistoryWorld(t)
	out := runAsAlice(t, deps, `
rela.output("hidden=" .. tostring(rela.version_token("SEC-1")))
rela.output("missing=" .. tostring(rela.version_token("NOPE-1")))
local tok = rela.version_token("P-1")
rela.output("token=" .. tostring(tok ~= nil and #tok > 0))
rela.output("stable=" .. tostring(tok == rela.version_token("P-1")))
local h = rela.history("P-1")
rela.output("tags=" .. #h[1].tags .. "," .. table.concat(h[2].tags, "+"))
`)
	for _, want := range []string{"hidden=nil", "missing=nil", "token=true", "stable=true", "tags=0,"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q lacks %q", out, want)
		}
	}
}
