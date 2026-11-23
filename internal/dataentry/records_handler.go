package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

const (
	recordsPath = "/api/v1/_records"
	// recordsBodyLimit caps an append body: records.MaxBatch rows of a few
	// fields each.
	recordsBodyLimit = 1 << 20
)

// recordsHandler owns the record routes (TKT-J7Q9C4K): typed rows owned by
// an entity, outside the graph. Nil until [SetRecords], which is the
// "feature absent" signal; its methods accept a nil receiver.
type recordsHandler struct {
	svc *records.Service
	// appender backs rela.append_records in an action script. It appends
	// through svc, so a script passes the same owner check as a POST.
	appender lua.RecordAppender
	// rollups backs roll-ups over record types (TKT-VMSWSWP).
	//
	// Nil: accepted; such a roll-up is then left out.
	rollups visibility.RecordAggregator
}

// SetRecordRollups installs the reader of roll-ups over record types. It
// follows [SetRecords]; without it those roll-ups have no value.
//
// Nil: agg rejected.
func SetRecordRollups(a *App, agg visibility.RecordAggregator) error {
	if a.records == nil || agg == nil {
		return errors.New("dataentry: SetRecordRollups requires SetRecords and a reader")
	}
	a.records.rollups = agg
	return nil
}

// SetRecords installs the record service and the rela.append_records
// capability over it. Nil: both are rejected; a deployment without record
// types never calls this.
func SetRecords(a *App, svc *records.Service, appender lua.RecordAppender) error {
	if svc == nil || appender == nil {
		return errors.New("dataentry: SetRecords requires a record service and its script appender")
	}
	a.records = &recordsHandler{svc: svc, appender: appender}
	return nil
}

// scriptAppender returns the rela.append_records capability. Nil: returned
// when no record service is installed, and then the binding is absent.
func (h *recordsHandler) scriptAppender() lua.RecordAppender {
	if h == nil {
		return nil
	}
	return h.appender
}

// handleV1Records serves every route under /api/v1/_records:
//
//	GET  /_records                          the declared record types
//	GET  /_records/{type}?owner=            one page of an owner's rows
//	POST /_records/{type}                   append rows: {"rows": [...]}
//	GET  /_records/{type}/_aggregate        fold rows into points
//	GET  /_records/{type}/_transaction?group=  one ledger transaction
//	POST /_records/{type}/_reverse          reverse one: {"group": "..."}
//	PATCH /_records/{type}/{id}             change a row of a mutable type
//	DELETE /_records/{type}/{id}?revision=  delete a row of a mutable type
//
// A row is readable when its owner is. A hidden owner is the same 404 as a
// missing one, and an aggregate folds readable owners only.
//
//archplan:auth=required
func (h *recordsHandler) handleV1Records(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.svc == nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, recordsPath), "/")
	var parts []string
	if rest != "" {
		parts = strings.Split(rest, "/")
	}
	switch {
	case len(parts) == 0:
		if allowMethod(w, r, http.MethodGet) {
			writeV1JSON(w, http.StatusOK, map[string]any{"types": h.svc.Types()})
		}
	case len(parts) == 1:
		switch r.Method {
		case http.MethodGet:
			h.list(w, r, parts[0])
		case http.MethodPost:
			h.append(w, r, parts[0])
		default:
			w.Header().Set("Allow", "GET, POST")
			writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		}
	case len(parts) == 2 && parts[1] == "_aggregate":
		if allowMethod(w, r, http.MethodGet) {
			h.aggregate(w, r, parts[0])
		}
	case len(parts) == 2 && parts[1] == "_transaction":
		if allowMethod(w, r, http.MethodGet) {
			rows, err := h.svc.Transaction(r.Context(), parts[0], r.URL.Query().Get("group"))
			h.writeRows(w, r, http.StatusOK, rows, err)
		}
	case len(parts) == 2 && parts[1] == "_reverse":
		if allowMethod(w, r, http.MethodPost) {
			h.reverse(w, r, parts[0])
		}
	case len(parts) == 2 && !strings.HasPrefix(parts[1], "_"):
		h.row(w, r, parts[0], parts[1])
	default:
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
	}
}

func (h *recordsHandler) list(w http.ResponseWriter, r *http.Request, typ string) {
	q := r.URL.Query()
	from, to, err := parseRange(q.Get("from"), q.Get("to"))
	if err != nil {
		writeRecordsError(w, r, err)
		return
	}
	limit := 0
	if s := q.Get("limit"); s != "" {
		if limit, err = strconv.Atoi(s); err != nil || limit < 0 {
			writeRecordsError(w, r, fmt.Errorf("%w: limit is a whole number", records.ErrInvalid))
			return
		}
	}
	page, err := h.svc.List(r.Context(), typ, q.Get("owner"), records.ListQuery{
		From: from, To: to, After: q.Get("after"), Limit: limit,
	})
	if err != nil {
		writeRecordsError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, page)
}

// recordRow is one row of an append body.
type recordRow struct {
	Owner  string         `json:"owner"`
	TS     *time.Time     `json:"ts,omitempty"`
	Fields map[string]any `json:"fields"`
}

func (h *recordsHandler) append(w http.ResponseWriter, r *http.Request, typ string) {
	var body struct {
		Rows []recordRow `json:"rows"`
	}
	if err := decodeRecordsBody(w, r, &body); err != nil {
		writeRecordsError(w, r, err)
		return
	}
	in := make([]records.Input, len(body.Rows))
	for i, row := range body.Rows {
		in[i] = records.Input{Owner: row.Owner, Fields: row.Fields}
		if row.TS != nil {
			in[i].TS = *row.TS
		}
	}
	rows, err := h.svc.Append(r.Context(), typ, in)
	h.writeRows(w, r, http.StatusCreated, rows, err)
}

func (h *recordsHandler) reverse(w http.ResponseWriter, r *http.Request, typ string) {
	var body struct {
		Group string `json:"group"`
	}
	if err := decodeRecordsBody(w, r, &body); err != nil {
		writeRecordsError(w, r, err)
		return
	}
	rows, err := h.svc.Reverse(r.Context(), typ, body.Group)
	h.writeRows(w, r, http.StatusCreated, rows, err)
}

// row serves the changes to one row of a mutable type (TKT-ERO2X5G). Both
// name the revision the caller read, so a change made in between is a 409
// rather than lost.
func (h *recordsHandler) row(w http.ResponseWriter, r *http.Request, typ, id string) {
	switch r.Method {
	case http.MethodPatch:
		var body struct {
			Revision *int64         `json:"revision"`
			TS       *time.Time     `json:"ts,omitempty"`
			Fields   map[string]any `json:"fields"`
		}
		if err := decodeRecordsBody(w, r, &body); err != nil {
			writeRecordsError(w, r, err)
			return
		}
		if body.Revision == nil {
			writeRecordsError(w, r, fmt.Errorf("%w: revision is required", records.ErrInvalid))
			return
		}
		c := records.Change{Revision: *body.Revision, Fields: body.Fields}
		if body.TS != nil {
			c.TS = *body.TS
		}
		rec, err := h.svc.Update(r.Context(), typ, id, c)
		if err != nil {
			writeRecordsError(w, r, err)
			return
		}
		writeV1JSON(w, http.StatusOK, rec)
	case http.MethodDelete:
		rev, err := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
		if err != nil {
			writeRecordsError(w, r, fmt.Errorf("%w: revision is required", records.ErrInvalid))
			return
		}
		if err := h.svc.Delete(r.Context(), typ, id, rev); err != nil {
			writeRecordsError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "PATCH, DELETE")
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

func (h *recordsHandler) aggregate(w http.ResponseWriter, r *http.Request, typ string) {
	q := r.URL.Query()
	from, to, err := parseRange(q.Get("from"), q.Get("to"))
	if err != nil {
		writeRecordsError(w, r, err)
		return
	}
	bucket, err := records.ParseBucket(q.Get("bucket"))
	if err != nil {
		writeRecordsError(w, r, err)
		return
	}
	pts, err := h.svc.Aggregate(r.Context(), records.AggregateQuery{
		Type: typ, Owners: q["owner"], From: from, To: to, Field: q.Get("field"), Bucket: bucket,
		PerOwner: q.Get("per_owner") == "true",
	})
	if err != nil {
		writeRecordsError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, map[string]any{"points": pts})
}

func (h *recordsHandler) writeRows(
	w http.ResponseWriter, r *http.Request, status int, rows []records.Record, err error,
) {
	if err != nil {
		writeRecordsError(w, r, err)
		return
	}
	writeV1JSON(w, status, map[string]any{"records": rows})
}

// decodeRecordsBody reads a JSON body into v. Numbers stay exact: a
// decimal field never passes through a float.
func decodeRecordsBody(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, recordsBodyLimit))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: malformed JSON body", records.ErrInvalid)
	}
	return nil
}

// parseRange parses the RFC 3339 bounds of a range; empty is open.
func parseRange(from, to string) (start, end time.Time, err error) {
	var out [2]time.Time
	for i, s := range []string{from, to} {
		if s == "" {
			continue
		}
		t, perr := time.Parse(time.RFC3339Nano, s)
		if perr != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: from and to are RFC 3339 times", records.ErrInvalid)
		}
		out[i] = t
	}
	return out[0], out[1], nil
}

func writeRecordsError(w http.ResponseWriter, r *http.Request, err error) {
	// detail drops the sentinel prefixes, which the title already says.
	detail := func() string {
		msg := err.Error()
		for _, s := range []error{records.ErrInvalid, records.ErrUnbalanced, records.ErrClosed} {
			msg = strings.ReplaceAll(msg, s.Error()+": ", "")
		}
		return msg
	}
	switch {
	case errors.Is(err, context.Canceled):
		return
	case errors.Is(err, records.ErrNotFound):
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
	case errors.Is(err, records.ErrForbidden):
		writeV1Error(w, r, http.StatusForbidden, "forbidden", "You may not change the records of this entity", "")
	case errors.Is(err, records.ErrStale):
		writeV1Error(w, r, http.StatusConflict, "stale_revision",
			"The record changed since you read it", "read it again and retry")
	case errors.Is(err, records.ErrGroupExists):
		writeV1Error(w, r, http.StatusConflict, "transaction_exists", "This transaction id is already used", "")
	case errors.Is(err, records.ErrUnbalanced):
		writeV1Error(w, r, http.StatusUnprocessableEntity, "unbalanced", "The transaction does not balance", detail())
	case errors.Is(err, records.ErrClosed):
		writeV1Error(w, r, http.StatusUnprocessableEntity, "period_closed", "The period is closed", detail())
	case errors.Is(err, records.ErrInvalid):
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid records request", detail())
	default:
		slog.Warn("dataentry: records request failed", "err", err, "method", r.Method)
		writeV1Error(w, r, http.StatusInternalServerError, "records_failed", "Records request failed",
			"check server logs")
	}
}

// registerRecordsRoutes mounts /api/v1/_records. The handler is read at
// request time because [SetRecords] runs after the routes are registered. It
// is a function rather than an [App] method to keep App under its plimsoll
// method line.
func registerRecordsRoutes(mux *http.ServeMux, a *App) {
	serve := func(w http.ResponseWriter, r *http.Request) { a.records.handleV1Records(w, r) }
	mux.HandleFunc(recordsPath, serve)
	mux.HandleFunc(recordsPath+"/", serve)
}
