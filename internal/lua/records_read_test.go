package lua_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/records"
)

type fakeRecordReader struct {
	typ, owner string
	list       records.ListQuery
	agg        records.AggregateQuery
	page       records.Page
	pts        []records.Point
	err        error
	tag        any
}

func (f *fakeRecordReader) List(ctx context.Context, typ, owner string, q records.ListQuery) (records.Page, error) {
	f.typ, f.owner, f.list, f.tag = typ, owner, q, ctx.Value(ctxKey{})
	return f.page, f.err
}

func (f *fakeRecordReader) Aggregate(ctx context.Context, q records.AggregateQuery) ([]records.Point, error) {
	f.agg, f.tag = q, ctx.Value(ctxKey{})
	return f.pts, f.err
}

// runRecordReads runs src with f behind the record read bindings (none when nil).
func runRecordReads(t *testing.T, f lua.RecordReader, src string) (string, error) {
	t.Helper()
	deps, _ := newHistoryWorld(t)
	deps.RecordReads = f
	ctx := context.WithValue(connectorCtx(), ctxKey{}, "script")
	out, err := runOAuth(ctx, t, deps, lua.Capabilities{}, src)
	return strings.Trim(strings.TrimSpace(out), `"`), err
}

func TestListRecords_Binding(t *testing.T) {
	ts := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	f := &fakeRecordReader{page: records.Page{Next: "cur", Records: []records.Record{
		{ID: "r1", Type: "measurement", Owner: "MET-1", TS: ts, RecordedAt: ts, Group: "g",
			Fields: map[string]any{"value": "0.82", "n": int64(3), "ok": true}},
	}}}
	out, err := runRecordReads(t, f, `
local page, err = rela.list_records("measurement", {owner = "MET-1", from = "2026-10-01T00:00:00Z", limit = 5})
local r = page.records[1]
rela.output(#page.records .. " " .. page.next .. " " .. r.id .. " " .. r.ts .. " " .. r.fields.value .. " " ..
  r.fields.n .. " " .. tostring(r.fields.ok) .. " " .. r.group .. " " .. tostring(err))`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "1 cur r1 2026-11-01T00:00:00Z 0.82 3 true g nil" {
		t.Errorf("output %q", out)
	}
	if f.typ != "measurement" || f.owner != "MET-1" || f.list.Limit != 5 ||
		!f.list.From.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {

		t.Errorf("query %+v", f.list)
	}
	if f.tag != "script" {
		t.Error("the reader did not get the script's context")
	}
}

func TestAggregateRecords_Binding(t *testing.T) {
	f := &fakeRecordReader{pts: []records.Point{{Owner: "MET-1", Count: 2, Sum: "3.5", Avg: "1.75"}}}
	out, err := runRecordReads(t, f, `
local pts, err = rela.aggregate_records("measurement",
  {owner = "MET-9", owners = {"MET-1"}, field = "value", bucket = "day", per_owner = true})
rela.output(#pts .. " " .. pts[1].owner .. " " .. pts[1].count .. " " .. pts[1].sum .. " " ..
  tostring(pts[1].start) .. " " .. tostring(err))`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "1 MET-1 2 3.5 nil nil" {
		t.Errorf("output %q", out)
	}
	if len(f.agg.Owners) != 2 || f.agg.Owners[0] != "MET-9" || f.agg.Owners[1] != "MET-1" ||
		f.agg.Field != "value" || f.agg.Bucket != records.BucketDay || !f.agg.PerOwner {

		t.Errorf("query %+v", f.agg)
	}
}

func TestRecordReads_ErrorIsReturned(t *testing.T) {
	f := &fakeRecordReader{err: errors.New("not found: MET-9")}
	out, err := runRecordReads(t, f, `
local a, e1 = rela.list_records("measurement", {owner = "MET-9"})
local b, e2 = rela.aggregate_records("measurement")
rela.output(tostring(a) .. " " .. e1 .. " " .. tostring(b) .. " " .. e2)`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "nil not found: MET-9 nil not found: MET-9" {
		t.Errorf("output %q", out)
	}
}

func TestRecordReads_Misuse(t *testing.T) {
	tests := []struct{ name, src, want string }{
		{"no owner", `rela.list_records("m", {})`, "owner is required"},
		{"unknown option", `rela.list_records("m", {owner = "A", ownr = "B"})`, "unknown option ownr"},
		{"bad time", `rela.list_records("m", {owner = "A", from = "yesterday"})`, "from must be an RFC 3339 string"},
		{"bad limit", `rela.list_records("m", {owner = "A", limit = -1})`, "limit must be a whole number"},
		{"bad owners", `rela.aggregate_records("m", {owners = {1}})`, "owners must be a list of strings"},
		{"bad flag", `rela.aggregate_records("m", {per_owner = "yes"})`, "per_owner must be a boolean"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runRecordReads(t, &fakeRecordReader{}, tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRecordReads_AbsentWithoutReader(t *testing.T) {
	_, err := runRecordReads(t, nil, `rela.list_records("m", {owner = "A"})`)
	if err == nil {
		t.Error("want an error calling an absent binding")
	}
}
