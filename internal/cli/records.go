package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/records"
)

// RecordsCmd groups the record verbs (TKT-EV8U8T4). Record types are typed
// rows owned by an entity and kept outside the graph.
//
// Every verb goes through the record service, as the REST routes do. append,
// list and aggregate act as the caller: they need update access on an owner
// to append, and read access on it to list or fold. expire is the operator's
// maintenance verb. It drops the rows past each type's retention, as the
// hourly job does, and it is only a CLI verb: no MCP tool, script binding or
// REST route runs it, so an agent or a web user cannot trigger it. It reads
// no owner and returns only a count.
type RecordsCmd struct {
	Append    RecordsAppendCmd    `cmd:"" help:"Append rows of a record type."`
	List      RecordsListCmd      `cmd:"" help:"List one owner's rows of a record type."`
	Aggregate RecordsAggregateCmd `cmd:"" help:"Fold rows of a record type into points."`
	Expire    RecordsExpireCmd    `cmd:"" help:"Drop the rows past each type's retention (operator maintenance)."`
}

// recordService is the record capability the records commands use. The
// wiring site passes the project's *records.Service.
type recordService interface {
	Append(ctx context.Context, typ string, in []records.Input) ([]records.Record, error)
	List(ctx context.Context, typ, owner string, q records.ListQuery) (records.Page, error)
	Aggregate(ctx context.Context, q records.AggregateQuery) ([]records.Point, error)
	Expire(ctx context.Context) (int64, error)
}

// recordServices is the binding the records commands take.
//
// Nil Svc: accepted; the schema declares no record types, and every command
// says so.
type recordServices struct {
	Svc recordService
}

// errNoRecordTypes answers a records command in a project without record
// types.
var errNoRecordTypes = errors.New("the project declares no record types")

// maxRecordsFile bounds the rows file: records.MaxBatch rows of a few fields.
const maxRecordsFile = 8 << 20

// recordsStdin is where `--file -` reads; a variable so tests can feed it.
var recordsStdin io.Reader = os.Stdin

func (r *recordServices) svc() (recordService, error) {
	if r == nil || r.Svc == nil {
		return nil, errNoRecordTypes
	}
	return r.Svc, nil
}

// recordsFailed shows a failure the caller caused as its text. Any other is
// logged and shown without the backend's text.
func recordsFailed(ctx context.Context, op string, err error) error {
	if text, ok := records.UserText(err); ok {
		return errors.New(text)
	}
	p := principal.From(ctx)
	slog.Warn("cli: records operation failed", "op", op, "user", p.User, "tool", p.Tool, "err", err)
	return fmt.Errorf("records %s failed; check the log", op)
}

func writeRecordsJSON(v any) error {
	enc := json.NewEncoder(out.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// RecordsAppendCmd appends rows. Give one row with --owner (and --ts and
// --field), or many with --file.
type RecordsAppendCmd struct {
	Type  string   `arg:"" help:"Record type key."`
	Owner string   `help:"Owner entity ID of the row."`
	TS    string   `name:"ts" help:"When the row applies, an RFC 3339 time. Default: now."`
	Field []string `short:"F" sep:"none" placeholder:"KEY=VALUE" help:"A field of the row (repeatable). Commas are kept."`
	File  string   `type:"path" help:"JSON file of rows, or - for stdin: [{\"owner\":..,\"ts\":..,\"fields\":{..}}] or {\"rows\":[..]}."`
}

// Run appends the rows and prints them as stored.
func (c *RecordsAppendCmd) Run(ctx context.Context, r *recordServices) error {
	svc, err := r.svc()
	if err != nil {
		return err
	}
	in, err := c.inputs()
	if err != nil {
		return err
	}
	stored, err := svc.Append(ctx, c.Type, in)
	if err != nil {
		return recordsFailed(ctx, "append", err)
	}
	if out.Format == output.FormatJSON {
		return writeRecordsJSON(map[string]any{"records": stored})
	}
	out.WriteSuccess("Appended %d rows to %s.", len(stored), c.Type)
	writeRecordTable(stored)
	return nil
}

// inputs reads the rows from the flags or the file; exactly one source.
func (c *RecordsAppendCmd) inputs() ([]records.Input, error) {
	single := c.Owner != "" || c.TS != "" || len(c.Field) > 0
	switch {
	case c.File != "" && single:
		return nil, errors.New("give one row with --owner, --ts and --field, or many with --file, not both")
	case c.File != "":
		return readRecordsFile(c.File)
	case c.Owner == "":
		return nil, errors.New("--owner is required (or --file for many rows)")
	}
	in := records.Input{Owner: strings.TrimSpace(c.Owner), Fields: map[string]any{}}
	if c.TS != "" {
		ts, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(c.TS))
		if err != nil {
			return nil, errors.New("--ts: an RFC 3339 time")
		}
		in.TS = ts
	}
	for _, f := range c.Field {
		k, v, err := parsePropertyFlag(f)
		if err != nil {
			return nil, err
		}
		in.Fields[k] = v
	}
	return []records.Input{in}, nil
}

// readRecordsFile reads rows from path, or stdin for "-". Numbers stay exact:
// a decimal field never passes through a float.
func readRecordsFile(path string) ([]records.Input, error) {
	src := recordsStdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("--file: %w", err)
		}
		defer f.Close()
		src = f
	}
	raw, err := io.ReadAll(io.LimitReader(src, maxRecordsFile+1))
	if err != nil {
		return nil, fmt.Errorf("--file: %w", err)
	}
	if len(raw) > maxRecordsFile {
		return nil, fmt.Errorf("--file: more than %d bytes", maxRecordsFile)
	}
	type row struct {
		Owner  string         `json:"owner"`
		TS     *time.Time     `json:"ts"`
		Fields map[string]any `json:"fields"`
	}
	var rows []row
	raw = bytes.TrimSpace(raw)
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if bytes.HasPrefix(raw, []byte("{")) {
		var body struct {
			Rows []row `json:"rows"`
		}
		err = dec.Decode(&body)
		rows = body.Rows
	} else {
		err = dec.Decode(&rows)
	}
	if err != nil {
		return nil, errors.New("--file: rows are [{owner, ts, fields}] or {\"rows\": [...]}")
	}
	in := make([]records.Input, len(rows))
	for i, r := range rows {
		in[i] = records.Input{Owner: r.Owner, Fields: r.Fields}
		if r.TS != nil {
			in[i].TS = *r.TS
		}
	}
	return in, nil
}

// RecordsListCmd lists one owner's rows.
type RecordsListCmd struct {
	Type     string `arg:"" help:"Record type key."`
	Owner    string `arg:"" help:"Owner entity ID."`
	Since    string `name:"from" help:"Rows at or after this RFC 3339 time."`
	Until    string `name:"to" help:"Rows before this RFC 3339 time."`
	Cursor   string `name:"after" help:"Cursor: the next value of the previous page."`
	PageSize int    `name:"limit" help:"Page size, 1 to 1000. Default 100."`
}

// Run prints one page of rows, oldest first.
func (c *RecordsListCmd) Run(ctx context.Context, r *recordServices) error {
	svc, err := r.svc()
	if err != nil {
		return err
	}
	from, to, err := recordsRange(c.Since, c.Until)
	if err != nil {
		return err
	}
	page, err := svc.List(ctx, c.Type, strings.TrimSpace(c.Owner), records.ListQuery{
		From: from, To: to, After: c.Cursor, Limit: c.PageSize,
	})
	if err != nil {
		return recordsFailed(ctx, "list", err)
	}
	if out.Format == output.FormatJSON {
		return writeRecordsJSON(page)
	}
	if len(page.Records) == 0 {
		out.WriteMessage("No rows.")
		return nil
	}
	writeRecordTable(page.Records)
	if page.Next != "" {
		out.WriteMessage("next: %s", page.Next)
	}
	return nil
}

// RecordsAggregateCmd folds rows into points.
type RecordsAggregateCmd struct {
	Type        string   `arg:"" help:"Record type key."`
	OwnerIDs    []string `name:"owner" help:"Owner entity ID to fold (repeatable). Default: every owner you may read."`
	Column      string   `name:"field" help:"Decimal or integer field to fold. Default: count rows only."`
	Granularity string   `name:"bucket" help:"Group by time: hour, day, week or month."`
	KeepApart   bool     `name:"per-owner" help:"Keep owners apart instead of folding them together."`
	Since       string   `name:"from" help:"Rows at or after this RFC 3339 time."`
	Until       string   `name:"to" help:"Rows before this RFC 3339 time."`
}

// Run prints the folded points. Only owners the caller may read fold.
func (c *RecordsAggregateCmd) Run(ctx context.Context, r *recordServices) error {
	svc, err := r.svc()
	if err != nil {
		return err
	}
	from, to, err := recordsRange(c.Since, c.Until)
	if err != nil {
		return err
	}
	bucket, err := records.ParseBucket(c.Granularity)
	if err != nil {
		return recordsFailed(ctx, "aggregate", err)
	}
	pts, err := svc.Aggregate(ctx, records.AggregateQuery{
		Type: c.Type, Owners: c.OwnerIDs, From: from, To: to, Field: c.Column, Bucket: bucket, PerOwner: c.KeepApart,
	})
	if err != nil {
		return recordsFailed(ctx, "aggregate", err)
	}
	if out.Format == output.FormatJSON {
		return writeRecordsJSON(map[string]any{"points": pts})
	}
	if len(pts) == 0 {
		out.WriteMessage("No rows.")
		return nil
	}
	tw := tabwriter.NewWriter(out.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "OWNER\tSTART\tCOUNT\tSUM\tMIN\tMAX\tAVG\tLATEST")
	for _, p := range pts {
		start := ""
		if !p.Start.IsZero() {
			start = p.Start.UTC().Format(time.RFC3339)
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n", p.Owner, start, p.Count, p.Sum, p.Min, p.Max, p.Avg, p.Latest)
	}
	return tw.Flush()
}

// RecordsExpireCmd drops the rows past retention.
type RecordsExpireCmd struct{}

// Run drops the expired rows and prints how many.
func (c *RecordsExpireCmd) Run(ctx context.Context, r *recordServices) error {
	svc, err := r.svc()
	if err != nil {
		return err
	}
	n, err := svc.Expire(ctx)
	if err != nil {
		return recordsFailed(ctx, "expire", err)
	}
	if out.Format == output.FormatJSON {
		return writeRecordsJSON(map[string]int64{"expired": n})
	}
	out.WriteSuccess("Dropped %d rows past retention.", n)
	return nil
}

// recordsRange parses the from and to flags; empty is open.
func recordsRange(from, to string) (start, end time.Time, err error) {
	var res [2]time.Time
	for i, s := range []string{from, to} {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		t, perr := time.Parse(time.RFC3339Nano, s)
		if perr != nil {
			return time.Time{}, time.Time{}, errors.New("--from and --to: RFC 3339 times")
		}
		res[i] = t
	}
	return res[0], res[1], nil
}

// writeRecordTable prints rows as ID, owner, time and fields.
func writeRecordTable(rows []records.Record) {
	tw := tabwriter.NewWriter(out.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tOWNER\tTS\tFIELDS")
	for _, rec := range rows {
		keys := make([]string, 0, len(rec.Fields))
		for k := range rec.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = fmt.Sprintf("%s=%v", k, rec.Fields[k])
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", rec.ID, rec.Owner, rec.TS.UTC().Format(time.RFC3339), strings.Join(parts, " "))
	}
	_ = tw.Flush()
}
