package lua

import (
	"context"
	"log/slog"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// asSeenWriter is an optional capability of [WriteDeps.VersionTags]:
// entitymanager.VersionTags provides it. It lets rela.create_entity and
// rela.update_entity return the token of the row they wrote, as the
// script's reader sees it, and lets rela.update_entity take `expect`
// (TKT-SM20FG). A sync loop tags its base with that token, so a user edit
// landing between the write and the tag makes the tag conflict instead of
// becoming the base.
//
// The write still goes through the runtime's own EntityManager, so its
// field gate and elevation apply; the capability only prepares ctx.
type asSeenWriter interface {
	WriteAsSeen(
		ctx context.Context, expect store.EntityVersion,
		view func(context.Context, entity.Ref) (*entity.Entity, error),
	) (context.Context, func(*entity.Entity) (store.EntityVersion, error))
}

// asSeen prepares ctx for a token-returning write. token is nil when the
// script asked for no token (neither opts.expect nor opts.token), and when
// the runtime has no tag writer; the write then returns no token. Only an
// asked-for token costs anything: the capture makes the write run in a
// transaction and read the row back, and the token reads it twice more. A
// function, not a method: Runtime is at its plimsoll method cap.
func asSeen(
	ctx context.Context, r *Runtime, opts writeOpts,
) (writeCtx context.Context, token func(*entity.Entity) (store.EntityVersion, error)) {
	if opts.Expect == "" && !opts.Token {
		return ctx, nil
	}
	expect := opts.Expect
	w, ok := r.deps.VersionTags.(asSeenWriter)
	if !ok || r.deps.VisibleReader == nil {
		return ctx, nil
	}
	rd := r.deps.VisibleReader
	view := func(ctx context.Context, ref entity.Ref) (*entity.Entity, error) {
		return rd.GetAddress(ctx, ref.String())
	}
	return w.WriteAsSeen(ctx, expect, view)
}

// writeTokenValue is the third return value of a write binding: the token,
// or nil when none was asked for or the runtime has no tag writer.
//
// It runs after the write committed, so it never raises: a script that sees
// an error would assume nothing was written and might write again. A failed
// token read is logged and answers nil; a loop then has no token to tag
// with and merges again on its next run.
func writeTokenValue(
	ctx context.Context, binding string, token func(*entity.Entity) (store.EntityVersion, error),
	written *entity.Entity,
) lua.LValue {
	if token == nil {
		return lua.LNil
	}
	tok, err := token(written)
	if err != nil {
		slog.WarnContext(ctx, "lua: write committed but its token could not be read",
			"binding", binding, "entity", written.Ref().String(), "error", err)
		return lua.LNil
	}
	return lua.LString(string(tok))
}
