package lua

import (
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// EntityMarshaller builds the Lua `entity` table for one entity. It is the
// injectable seam over entity→Lua marshaling: the default is [DefaultMarshaller]
// (the eager, behavior-preserving [EntityToTable]); a recording variant
// ([NewRecordingMarshaller]) captures which properties a script actually reads,
// so a consumer can gate an outcome on the reader's visibility of exactly those
// fields.
//
// The seam exists so field-access capture is a swap-in STRATEGY, not a fork of
// the shared marshaling code — most call sites keep the default and are wholly
// unaffected.
type EntityMarshaller func(ls *lua.LState, e *entity.Entity) *lua.LTable

// DefaultMarshaller is the standard, behavior-preserving marshaller: it is
// exactly [EntityToTable]. Existing call sites that pass no strategy get this,
// so their behavior is unchanged.
func DefaultMarshaller(ls *lua.LState, e *entity.Entity) *lua.LTable {
	return EntityToTable(ls, e)
}

// FieldAccessRecord accumulates the (entityID, field) pairs a script touched
// through a recording marshaller's `entity` tables. It is keyed BY ENTITY so a
// script that reads several entities in one run (the triggering entity plus any
// pulled via rela.get_entity / list / trace in later PRs — all of which will
// marshal through the same record) keeps each entity's touched fields distinct.
// This is the provenance a consumer gates on: a violation is shown only if the
// requester can read every (entity, field) that produced it.
//
// A read of `entity.content` records the sentinel [FieldContent] — content is a
// value-bearing field a consumer may also want to gate on. `id`/`type`/`mod_time`
// are NOT recorded: they are never ACL-hideable (id/type are identity; mod_time
// is storage metadata).
//
// Safe for the single-threaded gopher-lua run; the mutex guards against a
// record shared across goroutines.
type FieldAccessRecord struct {
	mu       sync.Mutex
	byEntity map[string]map[string]struct{} // entityID → set of field names
}

// FieldContent is the sentinel key recorded when a script reads entity.content.
const FieldContent = "\x00content"

// NewFieldAccessRecord returns an empty record.
func NewFieldAccessRecord() *FieldAccessRecord {
	return &FieldAccessRecord{byEntity: map[string]map[string]struct{}{}}
}

func (r *FieldAccessRecord) mark(entityID, field string) {
	r.mu.Lock()
	fields := r.byEntity[entityID]
	if fields == nil {
		fields = map[string]struct{}{}
		r.byEntity[entityID] = fields
	}
	fields[field] = struct{}{}
	r.mu.Unlock()
}

// AccessedFor returns the field names recorded for entityID (unordered, nil if
// none). Includes [FieldContent] if content was read.
func (r *FieldAccessRecord) AccessedFor(entityID string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	fields := r.byEntity[entityID]
	if len(fields) == 0 {
		return nil
	}
	out := make([]string, 0, len(fields))
	for k := range fields {
		out = append(out, k)
	}
	return out
}

// Entities returns the entity ids that had any field recorded (unordered).
func (r *FieldAccessRecord) Entities() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.byEntity))
	for id := range r.byEntity {
		out = append(out, id)
	}
	return out
}

// Has reports whether (entityID, field) was recorded.
func (r *FieldAccessRecord) Has(entityID, field string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	fields := r.byEntity[entityID]
	if fields == nil {
		return false
	}
	_, ok := fields[field]
	return ok
}

// NewRecordingMarshaller returns a marshaller that records every property a
// script reads into the returned record, plus the record itself.
//
// Gated-by-construction: the `properties` table is EMPTY, backed by an
// __index function. In Lua 5.1 (gopher-lua) __index fires on every read of an
// absent key — and every key is absent from an empty table — so
// `entity.properties.foo` always routes through the recorder. There is no raw
// data on the table to read around the gate. `entity:prop(name)` and
// `entity.content` are served by recording accessors for the same reason; a
// future accessor added WITHOUT wiring the recorder yields nil rather than raw
// data, so a coverage gap fails closed (loud absence) instead of leaking.
//
// Iteration caveat (documented, measured by the tests): gopher-lua's pairs()
// iterates a table's RAW contents and does not honor __index/__pairs, so
// `pairs(entity.properties)` over the empty proxy sees nothing. A script that
// enumerates properties therefore records nothing via that path — the
// fail-closed direction (reads nothing → can leak nothing), but a consumer that
// needs "a script that iterates depends on ALL props" must treat an iterating
// script conservatively. This marshaller does not itself decide that policy; it
// reports what was touched through the gated read paths.
func NewRecordingMarshaller() (EntityMarshaller, *FieldAccessRecord) {
	rec := NewFieldAccessRecord()
	m := func(ls *lua.LState, e *entity.Entity) *lua.LTable {
		return recordingEntityTable(ls, e, rec)
	}
	return m, rec
}

func recordingEntityTable(ls *lua.LState, e *entity.Entity, rec *FieldAccessRecord) *lua.LTable {
	t := ls.NewTable()
	t.RawSetString("id", lua.LString(e.ID))
	t.RawSetString("type", lua.LString(e.Type))

	if !e.UpdatedAt.IsZero() {
		t.RawSetString("mod_time", lua.LString(e.UpdatedAt.Format(time.RFC3339)))
	} else {
		t.RawSetString("mod_time", lua.LString(""))
	}

	// content is served through the entity table's own __index so the read is
	// recorded (a plain RawSetString would let the script read it ungated).
	// Every non-raw field EXCEPT the ones we set rawly above routes here.

	// props: an empty table whose __index records the key and returns the real
	// value. Empty ⇒ every field read is a miss ⇒ __index always fires.
	props := ls.NewTable()
	propsMeta := ls.NewTable()
	propsMeta.RawSetString("__index", ls.NewFunction(func(ls *lua.LState) int {
		key := ls.CheckString(2)
		rec.mark(e.ID, key)
		v, ok := e.Properties[key]
		if !ok {
			ls.Push(lua.LNil)
			return 1
		}
		ls.Push(GoToLuaValue(ls, v))
		return 1
	}))
	ls.SetMetatable(props, propsMeta)
	t.RawSetString("properties", props)

	// Recording prop(name, default): records name, reads from the Go map
	// directly (NOT via props.RawGetString, which would bypass recording AND
	// find nothing on the empty proxy).
	t.RawSetString("prop", ls.NewFunction(func(ls *lua.LState) int {
		name := ls.CheckString(2)
		defaultVal := ls.Get(3)
		rec.mark(e.ID, name)
		v, ok := e.Properties[name]
		if !ok {
			ls.Push(defaultVal)
			return 1
		}
		lv := GoToLuaValue(ls, v)
		if s, isStr := lv.(lua.LString); isStr && string(s) == "" {
			ls.Push(defaultVal)
			return 1
		}
		ls.Push(lv)
		return 1
	}))

	t.RawSetString("strip_prefix", ls.NewFunction(luaEntityStripPrefix))

	// content via the entity table's __index: record then serve. Set on a
	// metatable so a read of the absent raw `content` key routes here.
	tMeta := ls.NewTable()
	tMeta.RawSetString("__index", ls.NewFunction(func(ls *lua.LState) int {
		key := ls.CheckString(2)
		if key == "content" {
			rec.mark(e.ID, FieldContent)
			ls.Push(lua.LString(e.Content))
			return 1
		}
		ls.Push(lua.LNil)
		return 1
	}))
	ls.SetMetatable(t, tMeta)

	return t
}
