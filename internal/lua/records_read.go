// Lua bindings for reading records (TKT-EV8U8T4), next to rela.append_records.
//
//	rela.list_records("measurement", {owner = "MET-1", from = "...", to = "...", after = "...", limit = 100})
//	  -> {records = {{id, owner, ts, fields = {...}, ...}, ...}, next = "..."} | nil, err
//	rela.aggregate_records("measurement", {owners = {"MET-1"}, field = "value", bucket = "day", per_owner = true})
//	  -> {{owner, start, count, sum, min, max, avg, latest}, ...} | nil, err
//
// Both read as the script's principal through the record service, with the
// owner gate of the REST reads: a hidden owner's rows are not listed and not
// folded. Refusals and failures return nil and a message. A misuse of the
// arguments raises.
//
// Available on every runtime that has the service, readers included: they
// change nothing.
package lua

import (
	"context"
	"fmt"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/records"
)

// RecordReader reads rows for rela.list_records and rela.aggregate_records.
// Defined here at the consumer; the wiring site supplies the record service.
// It acts as the principal on ctx, and its errors are written for the script
// author.
type RecordReader interface {
	List(ctx context.Context, typ, owner string, q records.ListQuery) (records.Page, error)
	Aggregate(ctx context.Context, q records.AggregateQuery) ([]records.Point, error)
}

// registerRecordReads installs the record read bindings when the runtime has
// a reader.
func registerRecordReads(r *Runtime, rela *lua.LTable) {
	if r.deps.RecordReads == nil {
		return
	}
	b := recordReadBinding{reader: r.deps.RecordReads}
	r.L.SetField(rela, "list_records", r.L.NewFunction(b.list))
	r.L.SetField(rela, "aggregate_records", r.L.NewFunction(b.aggregate))
}

// recordReadBinding is the record read bindings over one reader.
type recordReadBinding struct {
	reader RecordReader
}

func (b recordReadBinding) list(ls *lua.LState) int {
	const binding = "rela.list_records"
	typ := ls.CheckString(1)
	var q records.ListQuery
	var owner string
	opts := ls.OptTable(2, ls.NewTable())
	if bad := readRecordOpts(opts, map[string]recordOpt{
		"owner": {str: &owner},
		"from":  {ts: &q.From},
		"to":    {ts: &q.To},
		"after": {str: &q.After},
		"limit": {num: &q.Limit},
	}); bad != "" {
		ls.RaiseError("%s: %s", binding, bad)
		return 0
	}
	if owner == "" {
		ls.RaiseError("%s: owner is required", binding)
		return 0
	}
	page, err := b.reader.List(scriptContext(ls), typ, owner, q)
	if err != nil {
		return pushRecordError(ls, err)
	}
	rows := ls.CreateTable(len(page.Records), 0)
	for _, rec := range page.Records {
		rows.Append(recordViewTable(ls, rec))
	}
	out := ls.NewTable()
	out.RawSetString("records", rows)
	if page.Next != "" {
		out.RawSetString("next", lua.LString(page.Next))
	}
	ls.Push(out)
	return 1
}

func (b recordReadBinding) aggregate(ls *lua.LState) int {
	const binding = "rela.aggregate_records"
	typ := ls.CheckString(1)
	q := records.AggregateQuery{Type: typ}
	opts := ls.OptTable(2, ls.NewTable())
	var owner, bucket string
	var owners *lua.LTable
	if bad := readRecordOpts(opts, map[string]recordOpt{
		"owner":     {str: &owner},
		"owners":    {list: &owners},
		"from":      {ts: &q.From},
		"to":        {ts: &q.To},
		"field":     {str: &q.Field},
		"bucket":    {str: &bucket},
		"per_owner": {flag: &q.PerOwner},
	}); bad != "" {
		ls.RaiseError("%s: %s", binding, bad)
		return 0
	}
	if owner != "" {
		q.Owners = append(q.Owners, owner)
	}
	if owners != nil {
		for i := 1; i <= owners.Len(); i++ {
			s, ok := owners.RawGetInt(i).(lua.LString)
			if !ok {
				ls.RaiseError("%s: owners must be a list of strings", binding)
				return 0
			}
			q.Owners = append(q.Owners, string(s))
		}
	}
	var err error
	if q.Bucket, err = records.ParseBucket(bucket); err != nil {
		return pushRecordError(ls, err)
	}
	pts, err := b.reader.Aggregate(scriptContext(ls), q)
	if err != nil {
		return pushRecordError(ls, err)
	}
	out := ls.CreateTable(len(pts), 0)
	for _, p := range pts {
		out.Append(recordPointTable(ls, p))
	}
	ls.Push(out)
	return 1
}

// pushRecordError returns nil and the message, the shape of every refusal.
func pushRecordError(ls *lua.LState, err error) int {
	msg := err.Error()
	if text, ok := records.UserText(err); ok {
		msg = text
	}
	ls.Push(lua.LNil)
	ls.Push(lua.LString(msg))
	return 2
}

// recordOpt names where one option of a read call goes. Exactly one field is
// set.
type recordOpt struct {
	str  *string
	ts   *time.Time
	num  *int
	flag *bool
	list **lua.LTable
}

// readRecordOpts copies the options table into dst. bad is the misuse to
// raise, or "". An unknown key is refused, so a misspelled option cannot
// silently widen a read.
func readRecordOpts(opts *lua.LTable, dst map[string]recordOpt) (bad string) {
	opts.ForEach(func(k, v lua.LValue) {
		if bad != "" {
			return
		}
		key, ok := k.(lua.LString)
		if !ok {
			bad = "options must be a table with string keys"
			return
		}
		o, known := dst[string(key)]
		if !known {
			bad = "unknown option " + string(key)
			return
		}
		bad = setRecordOpt(string(key), o, v)
	})
	return bad
}

func setRecordOpt(key string, o recordOpt, v lua.LValue) string {
	switch {
	case o.str != nil:
		s, ok := v.(lua.LString)
		if !ok {
			return key + " must be a string"
		}
		*o.str = strings.TrimSpace(string(s))
	case o.ts != nil:
		s, ok := v.(lua.LString)
		if !ok {
			return key + " must be an RFC 3339 string"
		}
		t, err := time.Parse(time.RFC3339Nano, string(s))
		if err != nil {
			return key + " must be an RFC 3339 string"
		}
		*o.ts = t
	case o.num != nil:
		n, ok := v.(lua.LNumber)
		if !ok || float64(n) != float64(int(n)) || n < 0 {
			return key + " must be a whole number"
		}
		*o.num = int(n)
	case o.flag != nil:
		b, ok := v.(lua.LBool)
		if !ok {
			return key + " must be a boolean"
		}
		*o.flag = bool(b)
	case o.list != nil:
		t, ok := v.(*lua.LTable)
		if !ok {
			return key + " must be a list"
		}
		*o.list = t
	}
	return ""
}

func recordViewTable(ls *lua.LState, rec records.Record) *lua.LTable {
	t := ls.NewTable()
	t.RawSetString("id", lua.LString(rec.ID))
	t.RawSetString("type", lua.LString(rec.Type))
	t.RawSetString("owner", lua.LString(rec.Owner))
	t.RawSetString("ts", lua.LString(rec.TS.UTC().Format(time.RFC3339Nano)))
	t.RawSetString("recorded_at", lua.LString(rec.RecordedAt.UTC().Format(time.RFC3339Nano)))
	setNonEmpty(t, "recorded_by", rec.RecordedBy)
	setNonEmpty(t, "group", rec.Group)
	setNonEmpty(t, "reverses", rec.Reverses)
	t.RawSetString("revision", lua.LNumber(rec.Revision))
	fields := ls.NewTable()
	for k, v := range rec.Fields {
		switch x := v.(type) {
		case string:
			fields.RawSetString(k, lua.LString(x))
		case int64:
			fields.RawSetString(k, lua.LNumber(x))
		case bool:
			fields.RawSetString(k, lua.LBool(x))
		default:
			fields.RawSetString(k, lua.LString(fmt.Sprint(x)))
		}
	}
	t.RawSetString("fields", fields)
	return t
}

func recordPointTable(ls *lua.LState, p records.Point) *lua.LTable {
	t := ls.NewTable()
	setNonEmpty(t, "owner", p.Owner)
	if !p.Start.IsZero() {
		t.RawSetString("start", lua.LString(p.Start.UTC().Format(time.RFC3339Nano)))
	}
	t.RawSetString("count", lua.LNumber(p.Count))
	setNonEmpty(t, "sum", p.Sum)
	setNonEmpty(t, "min", p.Min)
	setNonEmpty(t, "max", p.Max)
	setNonEmpty(t, "avg", p.Avg)
	setNonEmpty(t, "latest", p.Latest)
	return t
}

func setNonEmpty(t *lua.LTable, key, val string) {
	if val != "" {
		t.RawSetString(key, lua.LString(val))
	}
}
