// Lua bindings for version tags (TKT-VO6VG9).
//
//	rela.version_by_tag(addr, name)      -> entity table as of the tagged version | nil
//	rela.version_token(addr)             -> compare-and-set token of the current row | nil
//	rela.tag_version(addr, name [, opts]) -> version number | nil, "conflict"
//	rela.untag_version(addr, name)       -> true | false when the tag is absent
//
// opts is {expect = token} to tag the current state only while it is still
// the one token names, or {version = n} to tag version n. Without opts the
// current state is tagged unconditionally.
//
// The reads go through the runtime's VisibleReader, so a hidden entity reads
// as missing. The writes resolve the address through the same reader
// (resolveWriteTarget), so a hidden entity fails exactly like a missing one,
// and then reach the entitymanager, which authorizes and audits them.
//
// Tag writes are refused inside a store transaction (store.ErrTagInTx). No
// script runs inside one today, but a synchronous automation is not where a
// tag belongs: tag from a background action or a scheduled script. A runtime
// built without a tag writer (the synchronous automation cascade, document
// renders) does not register the write bindings at all.
//
// Free functions rather than Runtime methods: Runtime is at its plimsoll
// method cap.
package lua

import (
	"context"
	"errors"
	"math"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// VersionTagWriter sets and deletes version tags for the tag write bindings.
// Defined here at the consumer; entitymanager.VersionTags satisfies it. Its
// methods authorize, attribute and audit the write.
type VersionTagWriter interface {
	// TagCurrent's view is the caller's own read of ref; expect is compared
	// against the token of that read.
	TagCurrent(
		ctx context.Context, ref entity.Ref, name store.VersionTagName, expect store.EntityVersion,
		view func(context.Context, entity.Ref) (*entity.Entity, error),
	) (store.VersionMeta, error)
	TagVersion(ctx context.Context, ref entity.Ref, name store.VersionTagName, version int) (store.VersionMeta, error)
	UntagVersion(ctx context.Context, ref entity.Ref, name store.VersionTagName) error
}

// TaggedVersionReader is the part of a script read surface that resolves
// version tags. visibility.ScriptReader and visibility.UnrestrictedReader
// satisfy it. Optional: without it rela.version_by_tag raises as for a
// backend without history.
type TaggedVersionReader interface {
	VersionByTag(ctx context.Context, addr string, name store.VersionTagName) (*entity.Entity, store.VersionMeta, error)
}

// conflictResult is the second return value of rela.tag_version when the
// expected token no longer matches the current row.
const conflictResult = "conflict"

// versionTagBindings holds the runtime the tag bindings run in.
type versionTagBindings struct{ r *Runtime }

// registerVersionTagReadBindings installs rela.version_by_tag and
// rela.version_token.
func registerVersionTagReadBindings(r *Runtime, rela *lua.LTable) {
	vb := versionTagBindings{r: r}
	r.L.SetField(rela, "version_by_tag", r.L.NewFunction(vb.luaVersionByTag))
	r.L.SetField(rela, "version_token", r.L.NewFunction(vb.luaVersionToken))
}

// registerVersionTagWriteBindings installs rela.tag_version and
// rela.untag_version when the runtime has a tag writer.
func registerVersionTagWriteBindings(r *Runtime, rela *lua.LTable) {
	if r.deps.VersionTags == nil {
		return
	}
	vb := versionTagBindings{r: r}
	r.L.SetField(rela, "tag_version", r.L.NewFunction(vb.luaTagVersion))
	r.L.SetField(rela, "untag_version", r.L.NewFunction(vb.luaUntagVersion))
}

// checkTagName parses argument n as a tag name, raising on a bad one.
func checkTagName(ls *lua.LState, n int, binding string) (store.VersionTagName, bool) {
	name, err := store.ParseVersionTagName(ls.CheckString(n))
	if err != nil {
		ls.RaiseError("%s: %s", binding, err)
		return store.VersionTagName{}, false
	}
	return name, true
}

func (vb versionTagBindings) luaVersionByTag(ls *lua.LState) int {
	const binding = "rela.version_by_tag"
	addr := ls.CheckString(1)
	name, ok := checkTagName(ls, 2, binding)
	if !ok {
		return 0
	}
	rd, ok := vb.r.reader(ls, binding)
	if !ok {
		return 0
	}
	tr, ok := rd.(TaggedVersionReader)
	if !ok {
		ls.RaiseError("%s: %s", binding, store.ErrHistoryUnsupported)
		return 0
	}
	e, meta, err := tr.VersionByTag(vb.r.callerCtx(), addr, name)
	if err != nil {
		historyAnswer(ls, binding, err)
		return 1
	}
	t := EntityToTable(ls, e)
	t.RawSetString("version", lua.LNumber(meta.Version))
	ls.Push(t)
	return 1
}

// luaVersionToken returns store.VersionOf of the row the caller reads. The
// token is per reader: for a redacted read it covers only the fields the
// caller can see, and rela.tag_version compares it against the same reader's
// read (see entitymanager.VersionTags.TagCurrent).
func (vb versionTagBindings) luaVersionToken(ls *lua.LState) int {
	addr := ls.CheckString(1)
	rd, ok := vb.r.reader(ls, "rela.version_token")
	if !ok {
		return 0
	}
	e, err := rd.GetAddress(vb.r.callerCtx(), addr)
	if err != nil {
		ls.Push(lua.LNil)
		return 1
	}
	ls.Push(lua.LString(string(store.VersionOf(e))))
	return 1
}

// tagOpts is the parsed third argument of rela.tag_version.
type tagOpts struct {
	expect  store.EntityVersion
	version int
}

// checkTagOpts parses the optional opts table, raising on a malformed one.
func checkTagOpts(ls *lua.LState, binding string) (tagOpts, bool) {
	var o tagOpts
	if ls.GetTop() < 3 || ls.Get(3) == lua.LNil {
		return o, true
	}
	t := ls.CheckTable(3)
	if v := t.RawGetString("expect"); v != lua.LNil {
		s, ok := v.(lua.LString)
		if !ok || s == "" {
			ls.RaiseError("%s: expect must be a non-empty token string", binding)
			return o, false
		}
		o.expect = store.EntityVersion(s)
	}
	if v := t.RawGetString("version"); v != lua.LNil {
		num, ok := v.(lua.LNumber)
		n := int(num)
		if !ok || lua.LNumber(n) != num || n < 1 || n > math.MaxInt32 {
			ls.RaiseError("%s: version must be a positive integer", binding)
			return o, false
		}
		o.version = n
	}
	if o.expect != "" && o.version != 0 {
		ls.RaiseError("%s: pass expect or version, not both", binding)
		return o, false
	}
	return o, true
}

func (vb versionTagBindings) luaTagVersion(ls *lua.LState) int {
	const binding = "rela.tag_version"
	addr := ls.CheckString(1)
	name, ok := checkTagName(ls, 2, binding)
	if !ok {
		return 0
	}
	opts, ok := checkTagOpts(ls, binding)
	if !ok {
		return 0
	}
	ctx := vb.r.callerCtx()
	rd, ok := vb.r.reader(ls, binding)
	if !ok {
		return 0
	}
	ref, ok := resolveWriteTarget(ctx, ls, rd, addr)
	if !ok {
		return 0
	}
	var (
		meta store.VersionMeta
		err  error
	)
	if opts.version != 0 {
		meta, err = vb.r.deps.VersionTags.TagVersion(ctx, ref, name, opts.version)
	} else {
		// The token was minted by rela.version_token through this reader, so
		// the compare reads through it too.
		view := func(ctx context.Context, ref entity.Ref) (*entity.Entity, error) {
			return rd.GetAddress(ctx, ref.String())
		}
		meta, err = vb.r.deps.VersionTags.TagCurrent(ctx, ref, name, opts.expect, view)
	}
	var conflict *store.VersionConflictError
	if errors.As(err, &conflict) {
		ls.Push(lua.LNil)
		ls.Push(lua.LString(conflictResult))
		return 2
	}
	if err != nil {
		tagWriteError(ls, binding, addr, err)
		return 0
	}
	ls.Push(lua.LNumber(meta.Version))
	return 1
}

func (vb versionTagBindings) luaUntagVersion(ls *lua.LState) int {
	const binding = "rela.untag_version"
	addr := ls.CheckString(1)
	name, ok := checkTagName(ls, 2, binding)
	if !ok {
		return 0
	}
	ctx := vb.r.callerCtx()
	ref, ok := vb.writeTarget(ctx, ls, binding, addr)
	if !ok {
		return 0
	}
	err := vb.r.deps.VersionTags.UntagVersion(ctx, ref, name)
	switch {
	case err == nil:
		ls.Push(lua.LTrue)
	case !isEntityNotFound(err) && errors.Is(err, store.ErrNotFound):
		ls.Push(lua.LFalse)
	default:
		tagWriteError(ls, binding, addr, err)
		return 0
	}
	return 1
}

// writeTarget resolves addr to the face the tag write acts on, through the
// gated reader, so a hidden entity raises the same "entity not found" as a
// missing one.
func (vb versionTagBindings) writeTarget(
	ctx context.Context, ls *lua.LState, binding, addr string,
) (entity.Ref, bool) {
	rd, ok := vb.r.reader(ls, binding)
	if !ok {
		return entity.Ref{}, false
	}
	return resolveWriteTarget(ctx, ls, rd, addr)
}

// tagWriteError raises the script-facing error of a failed tag write. Like
// the other write bindings, the manager's error text is passed on: it names
// the denial, the refused name or the untaggable version.
func tagWriteError(ls *lua.LState, binding, addr string, err error) {
	switch {
	case isEntityNotFound(err):
		ls.RaiseError("entity not found: %s", addr)
	case errors.Is(err, store.ErrNotFound):
		ls.RaiseError("%s: no such version of %s", binding, addr)
	default:
		ls.RaiseError("%s: %s", binding, err)
	}
}
