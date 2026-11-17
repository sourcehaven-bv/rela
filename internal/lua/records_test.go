package lua_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

type fakeAppender struct {
	typ  string
	got  []lua.RecordRow
	tag  any
	err  error
	resp []lua.AppendedRecord
}

func (f *fakeAppender) AppendRecords(
	ctx context.Context, typ string, rows []lua.RecordRow,
) ([]lua.AppendedRecord, error) {
	f.typ = typ
	f.got = append(f.got, rows...)
	f.tag = ctx.Value(ctxKey{})
	return f.resp, f.err
}

// runAppendRecords runs src with f behind rela.append_records (none when nil).
func runAppendRecords(t *testing.T, f lua.RecordAppender, src string) (string, error) {
	t.Helper()
	deps, _ := newHistoryWorld(t)
	deps.Records = f
	ctx := context.WithValue(connectorCtx(), ctxKey{}, "script")
	out, err := runOAuth(ctx, t, deps, lua.Capabilities{}, src)
	return strings.Trim(strings.TrimSpace(out), `"`), err
}

func TestAppendRecords_Binding(t *testing.T) {
	ts := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	f := &fakeAppender{resp: []lua.AppendedRecord{
		{ID: "r1", Owner: "MET-1", TS: ts},
		{ID: "r2", Owner: "MET-2", TS: ts},
	}}
	out, err := runAppendRecords(t, f, `
local rows, err = rela.append_records("measurement", {
  {owner = "MET-1", ts = "2026-11-01T00:00:00Z", fields = {value = "0.82", note = "q4"}},
  {owner = "MET-2", fields = {value = 17, ok = true}},
})
rela.output(#rows .. " " .. rows[1].id .. " " .. rows[2].owner .. " " .. rows[1].ts .. " " .. tostring(err))`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "2 r1 MET-2 2026-11-01T00:00:00Z nil" {
		t.Errorf("output %q", out)
	}
	if f.typ != "measurement" || len(f.got) != 2 {
		t.Fatalf("type %q, rows %d", f.typ, len(f.got))
	}
	first, second := f.got[0], f.got[1]
	if first.Owner != "MET-1" || !first.TS.Equal(ts) || first.Fields["value"] != "0.82" || first.Fields["note"] != "q4" {
		t.Errorf("row 1 %+v", first)
	}
	// A row without ts leaves it zero: the service stamps the append time.
	if second.Owner != "MET-2" || !second.TS.IsZero() || second.Fields["value"] != int64(17) ||
		second.Fields["ok"] != true {
		t.Errorf("row 2 %+v", second)
	}
	if f.tag != "script" {
		t.Error("the appender did not get the script's context")
	}
}

func TestAppendRecords_ErrorIsReturned(t *testing.T) {
	f := &fakeAppender{err: errors.New("records: forbidden: update MET-1")}
	out, err := runAppendRecords(t, f, `
local rows, err = rela.append_records("measurement", {{owner = "MET-1", fields = {value = 1}}})
rela.output(tostring(rows) .. " " .. err)`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "nil records: forbidden: update MET-1" {
		t.Errorf("output %q", out)
	}
}

func TestAppendRecords_AbsentWithoutAppender(t *testing.T) {
	out, err := runAppendRecords(t, nil, `rela.output(tostring(rela.append_records))`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "nil" {
		t.Errorf("binding present without an appender: %q", out)
	}
}

// A reader runtime (validation rules, read-only surfaces) never carries the
// binding: its deps have no place for an appender.
func TestAppendRecords_AbsentOnReader(t *testing.T) {
	deps, _ := newHistoryWorld(t)
	var out bytes.Buffer
	rt := lua.NewReader(deps.ReadDeps, &out, lua.WithContext(connectorCtx()))
	defer rt.Close()
	if err := rt.RunString(`rela.output(tostring(rela.append_records))`); err != nil {
		t.Fatal(err)
	}
	if got := strings.Trim(strings.TrimSpace(out.String()), `"`); got != "nil" {
		t.Errorf("binding present on a reader: %q", got)
	}
}

func TestAppendRecords_RefusedInTx(t *testing.T) {
	deps, _ := newHistoryWorld(t)
	f := &fakeAppender{}
	deps.Records = f
	ctx := store.ContextInTx(connectorCtx())
	_, err := runOAuth(ctx, t, deps, lua.Capabilities{},
		`rela.append_records("measurement", {{owner = "MET-1", fields = {value = 1}}})`)
	if err == nil || !strings.Contains(err.Error(), "store transaction") {
		t.Fatalf("err = %v, want a transaction refusal", err)
	}
	if len(f.got) != 0 {
		t.Error("the appender ran inside a transaction")
	}
}

func TestAppendRecords_ArgumentMisuseRaises(t *testing.T) {
	big := `local rows = {} for i = 1, 1001 do rows[i] = {owner = "M", fields = {}} end
rela.append_records("m", rows)`
	for _, tc := range []struct{ name, src, want string }{
		{"no type", `rela.append_records(nil, {})`, "string expected"},
		{"no rows", `rela.append_records("m")`, "table expected"},
		{"empty rows", `rela.append_records("m", {})`, "non-empty list"},
		{"too many rows", big, "at most 1000 rows"},
		{"row not a table", `rela.append_records("m", {"x"})`, "row 1 must be a table"},
		{"positional key", `rela.append_records("m", {{"x"}})`, "string keys"},
		{"unknown key", `rela.append_records("m", {{owner = "M", face = "x"}})`, "unknown key face"},
		{"no owner", `rela.append_records("m", {{fields = {}}})`, "owner is required"},
		{"owner not a string", `rela.append_records("m", {{owner = 1}})`, "owner must be a string"},
		{"bad ts", `rela.append_records("m", {{owner = "M", ts = "yesterday"}})`, "RFC 3339"},
		{"ts not a string", `rela.append_records("m", {{owner = "M", ts = 3}})`, "RFC 3339"},
		{"fields not a table", `rela.append_records("m", {{owner = "M", fields = 3}})`, "fields must be a table"},
		{"nested field", `rela.append_records("m", {{owner = "M", fields = {v = {}}}})`, "field v must be"},
		{"positional field", `rela.append_records("m", {{owner = "M", fields = {1}}})`, "fields must be a table with string keys"},
		{"function field", `rela.append_records("m", {{owner = "M", fields = {v = print}}})`, "field v must be"},
		{"named key in rows", `local rows = {{owner = "M"}} rows.extra = {owner = "N"}
rela.append_records("m", rows)`, "without named keys"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAppender{}
			_, err := runAppendRecords(t, f, tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
			if len(f.got) != 0 {
				t.Error("the appender ran on a bad call")
			}
		})
	}
}
