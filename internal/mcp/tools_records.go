package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/records"
)

// RecordService is the record capability the record tools use. The wiring
// site passes the project's *records.Service, which gates every call on the
// owner: a row of a hidden owner is not listed and not folded, and an append
// needs update access on the owner (TKT-EV8U8T4).
type RecordService interface {
	Types() []records.TypeInfo
	Append(ctx context.Context, typ string, in []records.Input) ([]records.Record, error)
	List(ctx context.Context, typ, owner string, q records.ListQuery) (records.Page, error)
	Aggregate(ctx context.Context, q records.AggregateQuery) ([]records.Point, error)
}

// recordHandler serves the record tools.
type recordHandler struct {
	svc RecordService
}

func selRecords(h handlerSet) recordHandler { return h.records }

// registerRecordTools registers the record tools when [Deps.Records] is set.
func registerRecordTools(s *Server) {
	if s.deps().Records == nil {
		return
	}
	addTool(s, toolListRecordTypes(), recordTool(s, recordHandler.handleTypes))
	addTool(s, toolAppendRecords(), recordTool(s, recordHandler.handleAppend))
	addTool(s, toolListRecords(), recordTool(s, recordHandler.handleList))
	addTool(s, toolAggregateRecords(), recordTool(s, recordHandler.handleAggregate))
}

// recordTool binds a record handler. A reload can drop the record service
// after the tools were registered; the tools then answer that the project
// has no record types instead of calling a nil interface.
func recordTool(
	s *Server, method func(recordHandler, context.Context, toolRequest) *mcpgo.CallToolResult,
) mcpgo.ToolHandler {
	return bind(s, selRecords, func(
		h recordHandler, ctx context.Context, req *mcpgo.CallToolRequest,
	) (*mcpgo.CallToolResult, error) {
		if h.svc == nil {
			return errorResult("the project declares no record types"), nil
		}
		return method(h, ctx, newToolRequest(req)), nil
	})
}

func toolListRecordTypes() *mcpgo.Tool {
	return newTool("list_record_types",
		withDescription("List the record types: typed rows owned by an entity, kept outside the graph "+
			"(readings, bookings). Result: {types:[{key,label,owner,fields,field_order,retention,ledger,mutable}]}. "+
			"`owner` is the entity type the rows belong to"),
	)
}

func toolAppendRecords() *mcpgo.Tool {
	return newTool("append_records",
		withDescription("Append rows of one record type. The rows are stored together or not at all. "+
			"Every owner must be of the type's owner type, and you need update access to it. "+
			"Result: {records:[...]} as stored"),
		withString("type", required(), description("Record type key (see list_record_types)")),
		withArray("rows", required(), description(
			"At most 1000 rows, each {owner, ts, fields}. owner is the entity id. ts is an RFC 3339 time and "+
				"defaults to now. fields maps field names to string, number or boolean values; "+
				"write a decimal as a string to keep it exact")),
	)
}

func toolListRecords() *mcpgo.Tool {
	return newTool("list_records",
		withDescription("List one owner's rows of a record type, ordered by (ts, id). A hidden owner is "+
			"not found. Result: {records:[...],next}; pass `next` as `after` for the next page"),
		withString("type", required(), description("Record type key")),
		withString("owner", required(), description("Owner entity id")),
		withString("from", description("RFC 3339 time; rows at or after it")),
		withString("to", description("RFC 3339 time; rows before it")),
		withString("after", description("Cursor: the `next` of the previous page")),
		withNumber("limit", description("Page size, 1 to 1000 (default 100)")),
	)
}

func toolAggregateRecords() *mcpgo.Tool {
	return newTool("aggregate_records",
		withDescription("Fold rows of a record type into points: count, and for a numeric field sum, min, max, "+
			"avg and latest as exact decimal text. Folds only owners you may read. Result: {points:[...]}"),
		withString("type", required(), description("Record type key")),
		withArray("owners", description("Owner entity ids; omit to fold every readable owner with rows")),
		withString("field", description("Decimal or integer field to fold; omit to count rows only")),
		withString("bucket", description("Group by time"), enum("hour", "day", "week", "month")),
		withBoolean("per_owner", description("Keep owners apart instead of folding them together")),
		withString("from", description("RFC 3339 time; rows at or after it")),
		withString("to", description("RFC 3339 time; rows before it")),
	)
}

func (h recordHandler) handleTypes(_ context.Context, _ toolRequest) *mcpgo.CallToolResult {
	return jsonResult(map[string]any{"types": h.svc.Types()})
}

// appendRow is one row of append_records.
type appendRow struct {
	Owner  string         `json:"owner"`
	TS     *time.Time     `json:"ts"`
	Fields map[string]any `json:"fields"`
}

func (h recordHandler) handleAppend(ctx context.Context, args toolRequest) *mcpgo.CallToolResult {
	typ, err := args.RequireString("type")
	if err != nil {
		return errorResult(err.Error())
	}
	rows, err := appendRows(args)
	if err != nil {
		return errorResult(err.Error())
	}
	in := make([]records.Input, len(rows))
	for i, r := range rows {
		in[i] = records.Input{Owner: r.Owner, Fields: r.Fields}
		if r.TS != nil {
			in[i].TS = *r.TS
		}
	}
	stored, err := h.svc.Append(ctx, typ, in)
	if err != nil {
		return recordFailed(err)
	}
	return jsonResult(map[string]any{"records": stored})
}

// appendRows reads the rows argument. It re-reads the raw arguments with
// numbers kept as text, so a decimal never passes through a float.
func appendRows(args toolRequest) ([]appendRow, error) {
	if args.req == nil || args.req.Params == nil {
		return nil, errors.New("missing required argument: rows")
	}
	var body struct {
		Rows []appendRow `json:"rows"`
	}
	dec := json.NewDecoder(bytes.NewReader(args.req.Params.Arguments))
	dec.UseNumber()
	if err := dec.Decode(&body); err != nil {
		return nil, errors.New("rows: each row is {owner, ts, fields}")
	}
	if len(body.Rows) == 0 {
		return nil, errors.New("rows: at least one row is required")
	}
	return body.Rows, nil
}

func (h recordHandler) handleList(ctx context.Context, args toolRequest) *mcpgo.CallToolResult {
	typ, err := args.RequireString("type")
	if err != nil {
		return errorResult(err.Error())
	}
	owner, err := args.RequireString("owner")
	if err != nil {
		return errorResult(err.Error())
	}
	from, to, err := recordRange(args)
	if err != nil {
		return errorResult(err.Error())
	}
	page, err := h.svc.List(ctx, typ, trimID(owner), records.ListQuery{
		From: from, To: to, After: args.GetString("after", ""), Limit: args.GetInt("limit", 0),
	})
	if err != nil {
		return recordFailed(err)
	}
	return jsonResult(page)
}

func (h recordHandler) handleAggregate(ctx context.Context, args toolRequest) *mcpgo.CallToolResult {
	typ, err := args.RequireString("type")
	if err != nil {
		return errorResult(err.Error())
	}
	from, to, err := recordRange(args)
	if err != nil {
		return errorResult(err.Error())
	}
	bucket, err := records.ParseBucket(args.GetString("bucket", ""))
	if err != nil {
		return recordFailed(err)
	}
	owners := args.GetStringSlice("owners", nil)
	for i, o := range owners {
		owners[i] = trimID(o)
	}
	pts, err := h.svc.Aggregate(ctx, records.AggregateQuery{
		Type: typ, Owners: owners, From: from, To: to, Field: args.GetString("field", ""),
		Bucket: bucket, PerOwner: args.GetBool("per_owner", false),
	})
	if err != nil {
		return recordFailed(err)
	}
	return jsonResult(map[string]any{"points": pts})
}

// recordRange reads the from and to arguments; an absent one is open.
func recordRange(args toolRequest) (from, to time.Time, err error) {
	var out [2]time.Time
	for i, key := range []string{"from", "to"} {
		s := strings.TrimSpace(args.GetString(key, ""))
		if s == "" {
			continue
		}
		t, perr := time.Parse(time.RFC3339Nano, s)
		if perr != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("%s: an RFC 3339 time", key)
		}
		out[i] = t
	}
	return out[0], out[1], nil
}

// recordFailed answers a failed record request. A failure the caller caused
// shows its text; any other is logged and shown without it.
func recordFailed(err error) *mcpgo.CallToolResult {
	if text, ok := records.UserText(err); ok {
		return errorResult(text)
	}
	slog.Warn("mcp: records request failed", "err", err)
	return errorResult("records request failed; check the server log")
}
