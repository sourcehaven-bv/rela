// Lua binding for appending records (TKT-RK810V7).
//
//	rela.append_records("measurement", {
//	  {owner = "MET-1", fields = {value = "0.82"}},
//	  {owner = "MET-2", ts = "2026-11-01T00:00:00Z", fields = {value = 17}},
//	}) -> {{id, owner, ts}, ...} | nil, err
//
// The append runs as the script's principal through the record service, the
// same check as POST /api/v1/_records/{type}: every owner must be of the
// type's owner type, and the principal must be allowed to update every face
// of it. The rows are stored together or not at all. Refusals and failures
// return nil and a message; only a misuse of the arguments raises.
//
// Only writer runtimes carry it. A free function rather than a Runtime
// method: Runtime is at its plimsoll method cap.
package lua

import (
	"context"
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// maxAppendRecordRows bounds the rows one rela.append_records call takes. It
// matches the record service's batch limit, so a script hits the binding's
// clear message before the service's.
const maxAppendRecordRows = 1000

// RecordRow is one row of rela.append_records. TS is zero when the script
// names no time; the service then stamps the time of the append.
type RecordRow struct {
	Owner  string
	TS     time.Time
	Fields map[string]any
}

// AppendedRecord is one stored row as the script sees it.
type AppendedRecord struct {
	ID    string
	Owner string
	TS    time.Time
}

// RecordAppender appends rows for rela.append_records. Defined here at the
// consumer; the wiring site supplies the record service. It acts as the
// principal on ctx, and its errors are written for the script author.
type RecordAppender interface {
	AppendRecords(ctx context.Context, typ string, rows []RecordRow) ([]AppendedRecord, error)
}

// registerAppendRecords installs rela.append_records when the runtime has an
// appender.
func registerAppendRecords(r *Runtime, rela *lua.LTable) {
	if r.deps.Records == nil {
		return
	}
	b := appendRecordsBinding{appender: r.deps.Records}
	r.L.SetField(rela, "append_records", r.L.NewFunction(b.call))
}

// appendRecordsBinding is rela.append_records over one appender.
type appendRecordsBinding struct {
	appender RecordAppender
}

func (b appendRecordsBinding) call(ls *lua.LState) int {
	const binding = "rela.append_records"
	typ := ls.CheckString(1)
	rows, bad := parseRecordRows(ls.CheckTable(2))
	if bad != "" {
		ls.RaiseError("%s: %s", binding, bad)
		return 0
	}
	// The record service takes the store's write lock for the owner check and
	// the insert, which must not nest inside a transaction that holds it.
	ctx := scriptContext(ls)
	if store.InTx(ctx) {
		ls.RaiseError("%s: cannot run inside a store transaction; call it from an action, "+
			"a background automation, a flow step or a scheduled script", binding)
		return 0
	}
	out, err := b.appender.AppendRecords(ctx, typ, rows)
	if err != nil {
		ls.Push(lua.LNil)
		ls.Push(lua.LString(err.Error()))
		return 2
	}
	t := ls.CreateTable(len(out), 0)
	for _, rec := range out {
		row := ls.NewTable()
		row.RawSetString("id", lua.LString(rec.ID))
		row.RawSetString("owner", lua.LString(rec.Owner))
		row.RawSetString("ts", lua.LString(rec.TS.UTC().Format(time.RFC3339Nano)))
		t.Append(row)
	}
	ls.Push(t)
	return 1
}

// parseRecordRows reads the row list. It checks shape only: whether an owner
// and its fields are acceptable is the record service's to judge. bad is the
// misuse to raise, or "".
func parseRecordRows(list *lua.LTable) (rows []RecordRow, bad string) {
	n := list.Len()
	if n == 0 {
		return nil, "rows must be a non-empty list"
	}
	if n > maxAppendRecordRows {
		return nil, fmt.Sprintf("at most %d rows per call", maxAppendRecordRows)
	}
	rows = make([]RecordRow, 0, n)
	for i := 1; i <= n; i++ {
		t, ok := list.RawGetInt(i).(*lua.LTable)
		if !ok {
			return nil, fmt.Sprintf("row %d must be a table", i)
		}
		row, msg := parseRecordRow(t)
		if msg != "" {
			return nil, fmt.Sprintf("row %d: %s", i, msg)
		}
		rows = append(rows, row)
	}
	return rows, ""
}

func parseRecordRow(t *lua.LTable) (row RecordRow, bad string) {
	t.ForEach(func(k, v lua.LValue) {
		if bad != "" {
			return
		}
		key, ok := k.(lua.LString)
		if !ok {
			bad = "a row must be a table with string keys"
			return
		}
		switch key {
		case "owner":
			s, ok := v.(lua.LString)
			if !ok {
				bad = "owner must be a string"
				return
			}
			row.Owner = string(s)
		case "ts":
			s, ok := v.(lua.LString)
			if !ok {
				bad = "ts must be an RFC 3339 string"
				return
			}
			ts, err := time.Parse(time.RFC3339Nano, string(s))
			if err != nil {
				bad = "ts must be an RFC 3339 string"
				return
			}
			row.TS = ts
		case "fields":
			ft, ok := v.(*lua.LTable)
			if !ok {
				bad = "fields must be a table"
				return
			}
			row.Fields, bad = recordFields(ft)
		default:
			bad = "unknown key " + string(key)
		}
	})
	if bad == "" && row.Owner == "" {
		bad = "owner is required"
	}
	return row, bad
}

// recordFields converts a fields table to Go values. A field value is a
// scalar: the service refuses anything else, so a nested table is refused
// here with a clearer message.
func recordFields(t *lua.LTable) (map[string]any, string) {
	out := map[string]any{}
	bad := ""
	t.ForEach(func(k, v lua.LValue) {
		if bad != "" {
			return
		}
		key, ok := k.(lua.LString)
		if !ok {
			bad = "fields must be a table with string keys"
			return
		}
		if _, isTable := v.(*lua.LTable); isTable {
			bad = "field " + string(key) + " must be a string, number or boolean"
			return
		}
		out[string(key)] = luaValueToGo(v)
	})
	return out, bad
}
