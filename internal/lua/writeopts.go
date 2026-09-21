package lua

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// writeOpts is the parsed trailing options table shared by the create
// bindings. Zero value means "no options supplied", which is exactly the
// behavior every pre-existing call site had.
type writeOpts struct {
	// Face is the content state to write, already through the codec.
	Face entity.Face
	// Content is the relation body. A pointer so "absent" and "set to the
	// empty string" stay distinguishable, matching entity.RelationOptions.
	Content *string
}

// Known keys per binding. Package-level so the unknown-key rejection and its
// test read the SAME set — an allowlist the test restates by hand drifts from
// the parser it is meant to pin.
var (
	createEntityOptKeys   = []string{"face"}
	createRelationOptKeys = []string{"face", "content"}

	// Prebuilt lookup sets, so the rejection path does not rebuild a map per
	// write — these bindings run inside script loops.
	createEntityOptSet   = optKeySet(createEntityOptKeys)
	createRelationOptSet = optKeySet(createRelationOptKeys)
)

// optKeySet indexes an allowlist for membership testing.
func optKeySet(keys []string) map[string]struct{} {
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		set[k] = struct{}{}
	}
	return set
}

// parseWriteOpts reads an optional trailing options table at argument
// position pos, accepting only the keys in allowed.
//
// The argument may be absent, nil, or a table; **anything else raises**.
// That last rule is the load-bearing one. gopher-lua's GetTop() counts
// explicit trailing nils, so a present-but-wrong-typed argument is
// distinguishable from an omitted one, and the obvious "ignore it unless it
// is a table" shortcut would silently swallow
// `create_entity(t, props, body, nil, "draft")` — a caller passing the face
// as a bare string, which is the mistake a script author actually makes. A
// dropped face is BUG-HC6I2T's exact shape: the write lands on a row the
// caller did not name, and on a faced type the resulting `face_required`
// names an argument the caller believes they supplied.
//
// Unknown keys are rejected for the same reason `DisallowUnknownFields`
// guards the HTTP create body: `{fce = "draft"}` must not read as "no face".
//
// Nil: returns the zero writeOpts and a nil error when the argument is
// absent or nil.
func parseWriteOpts(
	s *lua.LState, pos int, allowed []string, allowedSet map[string]struct{},
) (writeOpts, error) {
	var o writeOpts
	if s.GetTop() < pos {
		return o, nil
	}
	switch arg := s.Get(pos).(type) {
	case *lua.LNilType:
		return o, nil
	case *lua.LTable:
		if err := readWriteOptKeys(arg, allowed, allowedSet, &o); err != nil {
			return writeOpts{}, err
		}
		return o, nil
	default:
		return writeOpts{}, fmt.Errorf("options must be a table, got %s", arg.Type())
	}
}

// readWriteOptKeys fills o from tbl, rejecting unknown keys and wrong-typed
// values rather than skipping either.
func readWriteOptKeys(
	tbl *lua.LTable, allowed []string, allowedSet map[string]struct{}, o *writeOpts,
) error {
	if err := rejectUnknownOptKeys(tbl, allowed, allowedSet); err != nil {
		return err
	}
	for _, key := range allowed {
		raw := tbl.RawGetString(key)
		if _, absent := raw.(*lua.LNilType); absent {
			continue
		}
		// Type-assert per key rather than once up front: both keys are
		// strings today, but hoisting the check would force the next
		// non-string option (a bool, a nested table) to either loosen it for
		// everyone or restructure this loop.
		switch key {
		case "face":
			str, ok := raw.(lua.LString)
			if !ok {
				return fmt.Errorf("option %q must be a string, got %s", key, raw.Type())
			}
			// Branch on key PRESENCE, never on emptiness: `{face = ""}` is a
			// caller naming a face that is not a valid one, and ParseFace
			// says so. Treating "" as absent (which the HTTP path does, for
			// its own reasons) would turn that into a silent default-face
			// write. ParseFace is also the only sanctioned constructor from
			// external input, and a script IS external input.
			face, err := entity.ParseFace(string(str))
			if err != nil {
				return fmt.Errorf("option %q: %w", key, err)
			}
			o.Face = face
		case "content":
			str, ok := raw.(lua.LString)
			if !ok {
				return fmt.Errorf("option %q must be a string, got %s", key, raw.Type())
			}
			body := string(str)
			o.Content = &body
		}
	}
	return nil
}

// rejectUnknownOptKeys fails on any key the binding does not accept.
func rejectUnknownOptKeys(
	tbl *lua.LTable, allowed []string, known map[string]struct{},
) error {
	var unknown error
	tbl.ForEach(func(k, _ lua.LValue) {
		if unknown != nil {
			return
		}
		name, ok := k.(lua.LString)
		if !ok {
			unknown = fmt.Errorf("option keys must be strings, got %s", k.Type())
			return
		}
		if _, allowedKey := known[string(name)]; !allowedKey {
			unknown = fmt.Errorf("unknown option %q; accepted: %s",
				string(name), strings.Join(allowed, ", "))
		}
	})
	return unknown
}
