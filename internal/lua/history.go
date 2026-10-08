// Lua bindings for entity version history (TKT-EC7F65).
//
//	rela.history(addr)        -> table of version rows, oldest first | nil
//	rela.get_version(addr, n) -> entity table as of version n | nil
//
// Both read through the runtime's VisibleReader, so an entity the script may
// not read answers nil, exactly as rela.get_entity does, and a snapshot is
// redacted for the caller. History exists on the database backends only. On
// any other backend both RAISE, for a missing id too: an empty answer there
// would read as "never changed", and a sync script comparing against a base
// version would then see every field as changed on both sides.
//
// Methods of historyBindings rather than of Runtime: Runtime is at its
// plimsoll method cap.
package lua

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// EntityVersionReader is the part of a script read surface that serves version
// history. visibility.ScriptReader and visibility.UnrestrictedReader satisfy
// it. Optional: a VisibleReader without it serves no history, and the
// bindings raise as for a backend that keeps none.
type EntityVersionReader interface {
	EntityVersions(ctx context.Context, addr string) ([]store.VersionMeta, error)
	EntityVersion(ctx context.Context, addr string, n int) (*entity.Entity, store.VersionMeta, error)
}

// historyBindings holds the runtime the history bindings read through.
type historyBindings struct{ r *Runtime }

// registerHistoryBindings installs rela.history and rela.get_version.
func registerHistoryBindings(r *Runtime, rela *lua.LTable) {
	hb := historyBindings{r: r}
	r.L.SetField(rela, "history", r.L.NewFunction(hb.luaHistory))
	r.L.SetField(rela, "get_version", r.L.NewFunction(hb.luaGetVersion))
}

// historyReader returns the runtime's history surface, raising when there is
// none.
func (hb historyBindings) historyReader(ls *lua.LState, binding string) (EntityVersionReader, bool) {
	rd, ok := hb.r.reader(ls, binding)
	if !ok {
		return nil, false
	}
	hr, ok := rd.(EntityVersionReader)
	if !ok {
		ls.RaiseError("%s: %s", binding, store.ErrHistoryUnsupported)
		return nil, false
	}
	return hr, true
}

// historyAnswer pushes nil for a miss and raises for every other error. A
// miss covers a hidden entity too, which must not be told apart from a
// missing one.
//
// A store error is logged, not passed on: its text can name tables and
// columns, and an action script's error can reach an HTTP client.
func historyAnswer(ls *lua.LState, binding string, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		ls.Push(lua.LNil)
	case errors.Is(err, store.ErrHistoryUnsupported):
		ls.RaiseError("%s: %s", binding, store.ErrHistoryUnsupported)
	default:
		slog.Warn("lua: history read failed", "binding", binding, "err", err)
		ls.RaiseError("%s: the version history could not be read", binding)
	}
}

func (hb historyBindings) luaHistory(ls *lua.LState) int {
	const binding = "rela.history"
	addr := ls.CheckString(1)
	hr, ok := hb.historyReader(ls, binding)
	if !ok {
		return 0
	}
	metas, err := hr.EntityVersions(hb.r.callerCtx(), addr)
	if err != nil {
		historyAnswer(ls, binding, err)
		return 1
	}
	t := ls.CreateTable(len(metas), 0)
	for _, m := range metas {
		t.Append(versionToTable(ls, m))
	}
	ls.Push(t)
	return 1
}

func (hb historyBindings) luaGetVersion(ls *lua.LState) int {
	const binding = "rela.get_version"
	addr := ls.CheckString(1)
	num := ls.CheckNumber(2)
	// Capped at the int32 maximum, the range a version column holds, so a
	// huge number is refused here rather than by the database.
	n := int(num)
	if lua.LNumber(n) != num || n < 1 || n > math.MaxInt32 {
		ls.RaiseError("%s: version must be a positive integer", binding)
		return 0
	}
	hr, ok := hb.historyReader(ls, binding)
	if !ok {
		return 0
	}
	e, meta, err := hr.EntityVersion(hb.r.callerCtx(), addr, n)
	if err != nil {
		historyAnswer(ls, binding, err)
		return 1
	}
	t := EntityToTable(ls, e)
	t.RawSetString("version", lua.LNumber(meta.Version))
	ls.Push(t)
	return 1
}

// versionToTable is one timeline row. Copy origins are left out: the HTTP
// API gates the source entity each one names, and no script needs them yet.
//
// The content hash is left out on purpose. It is a hash of the UNREDACTED
// snapshot, so a script that knows the visible fields could try the few
// values a hidden field can take and find the one that matches. The HTTP
// timeline does not serve it either.
func versionToTable(ls *lua.LState, m store.VersionMeta) *lua.LTable {
	t := ls.NewTable()
	t.RawSetString("version", lua.LNumber(m.Version))
	t.RawSetString("op", lua.LString(string(m.Op)))
	t.RawSetString("type", lua.LString(m.Type))
	t.RawSetString("face", lua.LString(m.Face.String()))
	t.RawSetString("created_at", lua.LString(m.CreatedAt.UTC().Format(time.RFC3339)))
	t.RawSetString("user", lua.LString(m.PrincipalUser))
	t.RawSetString("tool", lua.LString(m.PrincipalTool))
	t.RawSetString("triggered_by", lua.LString(m.TriggeredBy))
	t.RawSetString("prev_id", lua.LString(m.PrevID))
	return t
}
