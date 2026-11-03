package lua

import (
	"context"
	"errors"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// Page sizes of rela.get_mentioned_by, matching the REST endpoint.
const (
	mentionedByDefaultLimit = 50
	mentionedByMaxLimit     = 200
)

// mentionReader is the backlink read rela.get_mentioned_by needs. It is not
// part of [EntityReader]: the readers built by [visibility] provide it, and a
// reader without it makes the binding raise rather than answer empty.
type mentionReader interface {
	MentionedBy(ctx context.Context, target, after string, limit int) (visibility.MentionPage, error)
}

// mentionBindings holds rela.get_mentioned_by. It is not a [Runtime] method
// because Runtime sits at its plimsoll method cap.
type mentionBindings struct{ r *Runtime }

// getMentionedBy implements rela.get_mentioned_by(id, opts?) ->
// {items = {entity...}, next_cursor = string|nil, truncated = true|nil}
// (TKT-Q6AKUN).
//
// items are the entities the caller may read whose body mentions id as an
// `id` code span, gated and redacted by the runtime's reader, without their
// bodies. opts.limit (1 to 200, default 50) sizes the page; opts.cursor is a
// next_cursor from an earlier call. truncated is set when the scan stopped
// before reaching every source. A hidden or missing id answers an empty
// page, as rela.get_entity answers nil; any other fault raises.
func (b mentionBindings) getMentionedBy(ls *lua.LState) int {
	r := b.r
	id := ls.CheckString(1)
	if id == "" {
		ls.RaiseError("rela.get_mentioned_by: entity ID cannot be empty")
		return 0
	}
	limit, after, err := mentionedByOpts(ls.OptTable(2, nil))
	if err != nil {
		ls.RaiseError("rela.get_mentioned_by: %s", err.Error())
		return 0
	}
	rd, ok := r.reader(ls, "rela.get_mentioned_by")
	if !ok {
		return 0
	}
	mr, ok := rd.(mentionReader)
	if !ok {
		ls.RaiseError("rela.get_mentioned_by: this runtime's reader cannot read backlinks")
		return 0
	}

	ctx := r.callerCtx()
	result := ls.NewTable()
	items := ls.NewTable()
	result.RawSetString("items", items)
	target, err := rd.GetAddress(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		ls.Push(result)
		return 1
	}
	if err != nil {
		ls.RaiseError("rela.get_mentioned_by: %s", err.Error())
		return 0
	}
	page, err := mr.MentionedBy(ctx, target.ID, after, limit)
	if errors.Is(err, visibility.ErrMentionsUnsupported) { // coverage-ignore: every store backend keeps the index
		ls.Push(result)
		return 1
	}
	if err != nil {
		ls.RaiseError("rela.get_mentioned_by: %s", err.Error())
		return 0
	}
	for i, h := range page.Items {
		src := &entity.Entity{ID: h.ID, Type: h.Type, Face: h.Face, Properties: h.Properties, Redacted: h.Redacted}
		items.RawSetInt(i+1, EntityToTable(ls, src))
	}
	if page.Next != "" {
		result.RawSetString("next_cursor", lua.LString(page.Next))
	}
	if page.Truncated {
		result.RawSetString("truncated", lua.LTrue)
	}
	ls.Push(result)
	return 1
}

// mentionedByOpts reads opts.limit and opts.cursor. A value of the wrong
// type or out of range is an error, never a silent default.
func mentionedByOpts(opts *lua.LTable) (limit int, after string, err error) {
	limit = mentionedByDefaultLimit
	if opts == nil {
		return limit, after, nil
	}
	switch v := opts.RawGetString("limit").(type) {
	case *lua.LNilType:
	case lua.LNumber:
		n := int(v)
		if lua.LNumber(n) != v || n < 1 || n > mentionedByMaxLimit {
			return 0, after, errors.New("limit must be an integer from 1 to 200")
		}
		limit = n
	default:
		return 0, after, errors.New("limit must be a number")
	}
	switch v := opts.RawGetString("cursor").(type) {
	case *lua.LNilType:
	case lua.LString:
		if after, err = visibility.ParseMentionCursor(string(v)); err != nil {
			return 0, "", errors.New("cursor is not a value get_mentioned_by returned")
		}
	default:
		return 0, after, errors.New("cursor must be a string")
	}
	return limit, after, nil
}
