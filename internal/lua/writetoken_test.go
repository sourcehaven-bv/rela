package lua_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager/entitymanagertest"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// tokenTagWriter is a tag writer that also serves write tokens. tokenErr
// makes every token read fail.
type tokenTagWriter struct {
	recordingTagWriter
	prepared int
	tokenErr error
}

func (w *tokenTagWriter) WriteAsSeen(
	ctx context.Context, _ store.EntityVersion,
	_ func(context.Context, entity.Ref) (*entity.Entity, error),
) (writeCtx context.Context, token func(*entity.Entity) (store.EntityVersion, error)) {
	w.prepared++
	return ctx, func(*entity.Entity) (store.EntityVersion, error) {
		if w.tokenErr != nil {
			return "", w.tokenErr
		}
		return "tok", nil
	}
}

// patchingManager answers creates and patches with TKT-1 and records the
// write options, so a test can see whether external-ref writes were granted.
type patchingManager struct {
	entitymanagertest.PanicOnUse
	patches []entity.Patch
	creates []entity.CreateOptions
}

func (m *patchingManager) PatchEntity(_ context.Context, _ string, p entity.Patch) (*entity.UpdateResult, error) {
	m.patches = append(m.patches, p)
	return &entity.UpdateResult{Entity: &entity.Entity{ID: "TKT-1", Type: "ticket"}}, nil
}

func (m *patchingManager) CreateEntity(
	_ context.Context, _ *entity.Entity, opts entity.CreateOptions,
) (*entity.CreateResult, error) {
	m.creates = append(m.creates, opts)
	return &entity.CreateResult{Entity: &entity.Entity{ID: "TKT-1", Type: "ticket"}}, nil
}

// Finding 4: a write takes a token only when the script asks for one, and a
// token read that fails after the write committed answers nil rather than
// raising.
func TestWriteToken_OnlyWhenAsked(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		tokenErr     error
		wantPrepared int
		wantOut      string
	}{
		{
			name:    "update without opts takes no token",
			src:     `local e, _, tok = rela.update_entity("TKT-1", {title = "x"}); rela.output(e.id .. "," .. tostring(tok))`,
			wantOut: "TKT-1,nil",
		},
		{
			name:    "create without opts takes no token",
			src:     `local e, _, tok = rela.create_entity("ticket", {title = "x"}); rela.output(e.id .. "," .. tostring(tok))`,
			wantOut: "TKT-1,nil",
		},
		{
			name: "update asks for a token",
			src: `local e, _, tok = rela.update_entity("TKT-1", {title = "x"}, nil, {token = true})
rela.output(e.id .. "," .. tostring(tok))`,
			wantPrepared: 1, wantOut: "TKT-1,tok",
		},
		{
			name: "create asks for a token",
			src: `local e, _, tok = rela.create_entity("ticket", {title = "x"}, nil, nil, {token = true})
rela.output(e.id .. "," .. tostring(tok))`,
			wantPrepared: 1, wantOut: "TKT-1,tok",
		},
		{
			name: "a failed token read after the write answers nil",
			src: `local e, _, tok = rela.update_entity("TKT-1", {title = "x"}, nil, {token = true})
rela.output(e.id .. "," .. tostring(tok))`,
			tokenErr:     errors.New("read back failed"),
			wantPrepared: 1, wantOut: "TKT-1,nil",
		},
		{
			name: "a failed token read after an expect write answers nil",
			src: `local e, _, tok = rela.update_entity("TKT-1", {title = "x"}, nil, {expect = "t0"})
rela.output(e.id .. "," .. tostring(tok))`,
			tokenErr:     errors.New("read back failed"),
			wantPrepared: 1, wantOut: "TKT-1,nil",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := newHistoryWorld(t)
			w := &tokenTagWriter{tokenErr: tc.tokenErr}
			deps.VersionTags = w
			deps.EntityManager = &patchingManager{}
			out := runAsAlice(t, deps, tc.src)
			if !strings.Contains(out, tc.wantOut) {
				t.Errorf("output %q lacks %q", out, tc.wantOut)
			}
			if w.prepared != tc.wantPrepared {
				t.Errorf("token prepared %d times, want %d", w.prepared, tc.wantPrepared)
			}
		})
	}
}

func TestWriteToken_BadTokenOption(t *testing.T) {
	deps := newHistoryWorld(t)
	deps.VersionTags = &tokenTagWriter{}
	deps.EntityManager = &patchingManager{}
	err := runAsAliceErr(deps, `rela.update_entity("TKT-1", {title = "x"}, nil, {token = "yes"})`)
	if err == nil || !strings.Contains(err.Error(), "boolean") {
		t.Fatalf("err = %v, want a boolean refusal", err)
	}
}

// Finding 2: only a runtime built WithExternalRefWrites may write external
// refs; a plain writer runtime (what MCP lua_eval builds) may not.
func TestExternalRefWrites_OnlyWhenGranted(t *testing.T) {
	for _, granted := range []bool{false, true} {
		deps := newHistoryWorld(t)
		m := &patchingManager{}
		deps.EntityManager = m
		var opts []lua.Option
		if granted {
			opts = append(opts, lua.WithExternalRefWrites())
		}
		var out strings.Builder
		ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolScheduler})
		rt := lua.NewWriter(deps, &out, append(opts, lua.WithContext(ctx), lua.WithPrincipal(principal.From(ctx)))...)
		err := rt.RunString(`rela.update_entity("TKT-1", {title = "x"}); rela.create_entity("ticket", {title = "y"})`)
		rt.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(m.patches) != 1 || m.patches[0].WriteExternalRefs != granted {
			t.Errorf("granted=%v: patches = %+v", granted, m.patches)
		}
		if len(m.creates) != 1 || m.creates[0].WriteExternalRefs != granted {
			t.Errorf("granted=%v: creates = %+v", granted, m.creates)
		}
	}
}
