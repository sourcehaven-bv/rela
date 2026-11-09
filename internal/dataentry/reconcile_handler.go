package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/reconwire"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// reconcilePath is POST /api/v1/_reconcile (TKT-VP02WHG), the server side of
// rela-to-rela sync by set reconciliation (FEAT-CMVZQFI).
const reconcilePath = "/api/v1/_reconcile"

// The request limits. They are fixed, like the batch endpoint's.
const (
	// reconcileBodyLimit is the largest body read. 32 ranges with two
	// bounds of store.MaxRangeBound bytes each fit with room for the type
	// names.
	reconcileBodyLimit = 512 << 10
	// reconcileMaxRanges caps the ranges of one request.
	reconcileMaxRanges = 32
	// reconcileMaxTypeNames caps the type names over all ranges: each name
	// compiles the caller's read query and field scope for that type.
	reconcileMaxTypeNames = 256
	// reconcileMaxListRows caps the sum of the list limits of one request.
	reconcileMaxListRows = visibility.DefaultElementMaxRows
	// reconcileDeadline bounds the time one request runs its ranges. The
	// ranges it does not reach answer not_run.
	reconcileDeadline = 15 * time.Second
)

// reconcileHandler serves POST /api/v1/_reconcile: per range, the count and
// fingerprint of the elements the caller may read, or the elements
// themselves (TKT-VP02WHG).
//
// # Threat model
//
// The caller is a peer: an authenticated principal whose request body is
// entirely untrusted, including every range bound and type name. What it
// may learn is exactly what it could list through the read API: the rows
// it may read and the property values it may see. Three properties hold
// that:
//
//   - The scope is never taken from the body. Each range is answered by
//     [visibility.Elements], which compiles the store scope from the
//     acl.Request bound at the request edge, on every call. The body only
//     narrows: it names types and bounds, never grants. A hidden row is
//     absent from every count, fingerprint and list, so a deleted and a
//     hidden row look the same; a hidden field never reaches a view hash
//     (BUG-J01V0Q).
//   - Nothing is cached. The answer is one principal's and is built fresh;
//     the response says no-store.
//   - The work is bounded, because peer-chosen ranges are the attack
//     surface for cost: a token bucket per principal, a slot cap per server
//     and per principal, a deadline, and caps on ranges, type names, list
//     rows and body size. The row gate of a type with a relation predicate
//     costs the whole type per call (RR-3RVT441); the bucket and the slots
//     bound how often a peer can make the server pay it. Any range, a
//     fingerprint too, may load and redact up to
//     [visibility.DefaultElementMaxRows] rows when the caller has a hidden
//     field on its types, so one request loads at most reconcileMaxRanges
//     times that, and the slots bound how many requests do so at once. The
//     deadline is checked between ranges. A slot is held while the ranges
//     are read, not while the answer is written, so a peer that stops
//     reading holds none.
//
// Type names, relation meta gating and the limits are configuration, which
// is not secret, so their refusals name the type or the limit. Whether a
// range answers too_large depends on the scope and the row count, and, on
// a database backend that is still filling its stored elements, on whether
// some are pending; the count is the caller's to know and the backfill is
// no secret, so the same range may answer too_large and later ok. A
// fingerprint is not a commitment
// (see package reconcile): a writer in scope can make two different sets
// match, so a match is never evidence of integrity.
type reconcileHandler struct {
	schema   func() *Schema
	elements elementReader
	gate     *reconcileGate
	// deadline is reconcileDeadline; tests shorten it.
	deadline time.Duration
}

// elementReader is the gated element range read the endpoint serves.
// [visibleElements] implements it over [*visibility.Elements].
type elementReader interface {
	Sum(ctx context.Context, types visibility.ElementTypes, r store.ElementRange) (rangeSum, error)
	List(
		ctx context.Context, types visibility.ElementTypes, r store.ElementRange, limit int,
	) (visibility.ElementPage, error)
}

// rangeSum is a fingerprint answer as the endpoint reports it: the element
// count and the fingerprint in its wire form. The handler needs no range
// arithmetic, so it does not depend on package reconcile.
type rangeSum struct {
	count       uint64
	fingerprint string
}

// visibleElements adapts [*visibility.Elements] to [elementReader].
type visibleElements struct{ *visibility.Elements }

// Sum returns the count and the wire form of the fingerprint of the range.
func (v visibleElements) Sum(
	ctx context.Context, types visibility.ElementTypes, r store.ElementRange,
) (rangeSum, error) {
	rg, err := v.Fingerprint(ctx, types, r)
	if err != nil {
		return rangeSum{}, err
	}
	return rangeSum{count: rg.Count, fingerprint: rg.Sum.String()}, nil
}

// SetReconcile installs the element range read and so enables
// POST /api/v1/_reconcile. Call it before [App.NewRouter]. A package function
// rather than an App method because App is at its plimsoll method cap.
//
// Nil: rejected. A deployment that does not serve reconciliation never
// calls this; the endpoint then answers 501. The parameter is the concrete
// pointer so a nil *visibility.Elements is caught here.
func SetReconcile(a *App, el *visibility.Elements) error {
	if el == nil {
		return errors.New("dataentry: SetReconcile requires an element range read")
	}
	h, err := newReconcileHandler(a.State, visibleElements{el}, newReconcileGate(reconcileGateConfig{}))
	if err != nil {
		return err
	}
	a.reconcile = h
	return nil
}

// newReconcileHandler builds the handler.
//
// Nil: every argument rejected; a missing gate would admit unbounded work.
func newReconcileHandler(schema func() *Schema, el elementReader, gate *reconcileGate) (*reconcileHandler, error) {
	switch {
	case schema == nil:
		return nil, errors.New("dataentry: reconcile: schema is required")
	case el == nil:
		return nil, errors.New("dataentry: reconcile: element reader is required")
	case gate == nil:
		return nil, errors.New("dataentry: reconcile: admission gate is required")
	}
	return &reconcileHandler{schema: schema, elements: el, gate: gate, deadline: reconcileDeadline}, nil
}

// reconcileOp is one validated range of a request.
type reconcileOp struct {
	types visibility.ElementTypes
	r     store.ElementRange
	list  bool
	limit int
}

//archplan:auth=required
func (h *reconcileHandler) handleV1Reconcile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", r.Method)
		return
	}
	if h == nil {
		writeV1Error(w, r, http.StatusNotImplemented, "reconcile_unavailable",
			"Reconciliation is not available", "this server does not serve element ranges")
		return
	}
	// The body is read and checked before admission, as on _batch: both are
	// bounded and cheap, and a request refused here costs no slot.
	var req reconwire.Request
	if !decodeStrictBody(w, r, reconcileBodyLimit, "512 KiB", &req) {
		return
	}
	ops, err := planReconcile(h.schema().Meta, req)
	if err != nil {
		var re *reconcileRangeError
		if errors.As(err, &re) {
			writeReconcileRangeError(w, r, http.StatusBadRequest, "invalid_reconcile_request",
				"Invalid reconcile request", re.index, re.err)
			return
		}
		writeV1Error(w, r, http.StatusBadRequest, "invalid_reconcile_request", "Invalid reconcile request", err.Error())
		return
	}
	release, refusal := h.gate.acquire(principal.From(r.Context()).User)
	if refusal != nil {
		refusal.write(w, r)
		return
	}
	defer release()
	out, failed, err := h.run(r.Context(), ops)
	// The slot bounds the reads, not the write: a peer that stops reading
	// its response must not hold a slot.
	release()
	switch {
	case errors.Is(err, errReconcileClientGone):
		// No one is left to answer; the status is for the access log.
		w.WriteHeader(statusClientClosedRequest)
	case err != nil:
		writeReconcileError(w, r, failed, err)
	default:
		writeV1JSON(w, http.StatusOK, out)
	}
}

// statusClientClosedRequest is the status logged for a request whose client
// went away before the answer (the nginx convention).
const statusClientClosedRequest = 499

// errReconcileClientGone reports that the client's context ended a run.
var errReconcileClientGone = errors.New("dataentry: reconcile: client went away")

// run answers ops in order under the deadline. On an error that fails the
// request it returns the index of the range and the error. Every result is
// in memory before the caller writes, so no partial body leaves.
func (h *reconcileHandler) run(parent context.Context, ops []reconcileOp) (reconwire.Response, int, error) {
	ctx, cancel := context.WithTimeout(parent, h.deadline)
	defer cancel()
	out := reconwire.Response{Ranges: make([]reconwire.Result, 0, len(ops))}
	for i, op := range ops {
		if parent.Err() != nil {
			return out, i, errReconcileClientGone
		}
		if ctx.Err() != nil {
			out.Ranges = append(out.Ranges, reconwire.Result{Status: reconwire.StatusNotRun})
			continue
		}
		res, err := h.answer(ctx, op)
		switch {
		case err == nil:
			out.Ranges = append(out.Ranges, res)
		case errors.Is(err, visibility.ErrRangeTooLarge):
			out.Ranges = append(out.Ranges, reconwire.Result{Status: reconwire.StatusTooLarge})
		case parent.Err() != nil:
			return out, i, errReconcileClientGone
		case ctx.Err() != nil:
			// The deadline, not the client, ended the call.
			out.Ranges = append(out.Ranges, reconwire.Result{Status: reconwire.StatusNotRun})
		default:
			return out, i, err
		}
	}
	return out, 0, nil
}

// answer runs one range.
func (h *reconcileHandler) answer(ctx context.Context, op reconcileOp) (reconwire.Result, error) {
	if !op.list {
		rs, err := h.elements.Sum(ctx, op.types, op.r)
		if err != nil {
			return reconwire.Result{}, err
		}
		count := rs.count
		return reconwire.Result{Status: reconwire.StatusOK, Count: &count, Fingerprint: rs.fingerprint}, nil
	}
	page, err := h.elements.List(ctx, op.types, op.r, op.limit)
	if err != nil {
		return reconwire.Result{}, err
	}
	els := make([]reconwire.Element, len(page.Elements))
	for i, e := range page.Elements {
		els[i] = reconwire.Element{Address: e.Address, Hash: e.Hash.String()}
	}
	return reconwire.Result{Status: reconwire.StatusOK, Elements: els, Last: page.Last, More: page.More}, nil
}

// writeReconcileError answers a range that failed for a reason other than
// its size or the deadline. Nothing of the request is answered. A refusal
// of one range names it in the problem's errors, with the JSON pointer
// /ranges/<index>, so a client can split or skip that range without parsing
// the detail.
func writeReconcileError(w http.ResponseWriter, r *http.Request, index int, err error) {
	switch {
	case errors.Is(err, visibility.ErrRelationMetaGated):
		// The same for every principal: the policy can hide a meta key of
		// this relation type, so its elements cannot be served yet
		// (TKT-0RBFN0). The error text names the type; config is not secret.
		writeReconcileRangeError(w, r, http.StatusUnprocessableEntity, "relation_meta_gated",
			"Relation type not supported", index, err)
	case errors.Is(err, store.ErrInvalidQuery):
		writeReconcileRangeError(w, r, http.StatusBadRequest, "invalid_reconcile_request",
			"Invalid reconcile request", index, err)
	default:
		writeInternalError(w, r, "reconcile_failed", "Reconciliation failed", err)
	}
}

// reconcileRangeError is a planning error about one range of a request.
type reconcileRangeError struct {
	index int
	err   error
}

func (e *reconcileRangeError) Error() string { return fmt.Sprintf("range %d: %v", e.index, e.err) }
func (e *reconcileRangeError) Unwrap() error { return e.err }

// writeReconcileRangeError writes a problem about range index of the request.
func writeReconcileRangeError(
	w http.ResponseWriter, r *http.Request, status int, errType, title string, index int, err error,
) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v1.Error{
		Type:     "https://rela.dev/errors/" + errType,
		Title:    title,
		Status:   status,
		Detail:   fmt.Sprintf("range %d: %v", index, err),
		Instance: r.URL.Path,
		Errors: []v1.FieldError{{
			Source: v1.ErrorSource{Pointer: "/ranges/" + strconv.Itoa(index)}, Code: errType, Detail: err.Error(),
		}},
	})
}

// planReconcile checks req against the metamodel and the limits before
// anything is read, and returns the ranges to run.
func planReconcile(meta *metamodel.Metamodel, req reconwire.Request) ([]reconcileOp, error) {
	switch {
	case len(req.Ranges) == 0:
		return nil, errors.New("ranges must not be empty")
	case len(req.Ranges) > reconcileMaxRanges:
		return nil, fmt.Errorf("at most %d ranges per request", reconcileMaxRanges)
	}
	ops := make([]reconcileOp, len(req.Ranges))
	names, rows := 0, 0
	for i, rr := range req.Ranges {
		op, err := planReconcileRange(meta, rr)
		if err != nil {
			return nil, &reconcileRangeError{index: i, err: err}
		}
		names += len(rr.Entities) + len(rr.Relations)
		if names > reconcileMaxTypeNames {
			return nil, fmt.Errorf("at most %d type names over all ranges", reconcileMaxTypeNames)
		}
		rows += op.limit
		if rows > reconcileMaxListRows {
			return nil, fmt.Errorf("the list limits add up to more than %d rows", reconcileMaxListRows)
		}
		ops[i] = op
	}
	return ops, nil
}

func planReconcileRange(meta *metamodel.Metamodel, rr reconwire.Range) (reconcileOp, error) {
	var op reconcileOp
	switch rr.Kind {
	case "entity":
		if len(rr.Relations) > 0 {
			return op, errors.New("an entity range names no relation types")
		}
	case "relation":
		op.r.Relations = true
		if len(rr.Relations) == 0 {
			return op, errors.New("a relation range names at least one relation type")
		}
	default:
		return op, fmt.Errorf("kind %q is not entity or relation", rr.Kind)
	}
	if len(rr.Entities) == 0 {
		// A relation range needs them too: they are its endpoint scope, and
		// without them it would answer an empty range that reads as in sync.
		return op, errors.New("entities must name at least one entity type")
	}
	if err := checkTypeNames("entity", rr.Entities, func(n string) bool { _, ok := meta.Entities[n]; return ok }); err != nil {
		return op, err
	}
	if err := checkTypeNames("relation", rr.Relations, func(n string) bool { _, ok := meta.Relations[n]; return ok }); err != nil {
		return op, err
	}
	for _, n := range rr.Relations {
		// Like a range with no entity types, a relation type whose ends are
		// not in scope would answer an empty range that reads as in sync.
		def := meta.Relations[n]
		inScope := func(t string) bool { return slices.Contains(rr.Entities, t) }
		if !slices.ContainsFunc(def.From, inScope) || !slices.ContainsFunc(def.To, inScope) {
			return op, fmt.Errorf("relation type %q links %v to %v: entities must name a type of each end",
				n, def.From, def.To)
		}
	}
	op.types = visibility.ElementTypes{Entities: rr.Entities, Relations: rr.Relations}

	switch {
	case rr.After != nil && rr.Lo != "":
		return op, errors.New("lo and after are exclusive")
	case rr.After != nil:
		// No address holds NUL, so the first address after After is at
		// least After+"\x01".
		op.r.Lo = *rr.After + "\x01"
	default:
		op.r.Lo = rr.Lo
	}
	if rr.Hi == nil {
		op.r.Unbounded = true
	} else {
		op.r.Hi = *rr.Hi
	}
	if _, err := store.PlanElementRange(store.ElementScope{}, op.r); err != nil {
		return op, err
	}

	switch rr.Op {
	case "fingerprint":
		if rr.Limit != 0 {
			return op, errors.New("a fingerprint takes no limit")
		}
	case "list":
		if rr.Limit < 1 || rr.Limit > reconcileMaxListRows {
			return op, fmt.Errorf("limit %d is outside 1..%d", rr.Limit, reconcileMaxListRows)
		}
		op.list, op.limit = true, rr.Limit
	default:
		return op, fmt.Errorf("op %q is not fingerprint or list", rr.Op)
	}
	return op, nil
}

// checkTypeNames refuses an undeclared or repeated type name, naming it.
func checkTypeNames(kind string, names []string, declared func(string) bool) error {
	for i, n := range names {
		if !declared(n) {
			return fmt.Errorf("%s type %q is not declared", kind, n)
		}
		if slices.Contains(names[:i], n) {
			return fmt.Errorf("%s type %q is named twice", kind, n)
		}
	}
	return nil
}

// reconcileRefusal is an admission refusal: 429 for the rate limit, 503 for
// full slots, both with Retry-After.
type reconcileRefusal struct {
	status     int
	code       string
	title      string
	retryAfter time.Duration
}

func (f *reconcileRefusal) write(w http.ResponseWriter, r *http.Request) {
	secs := max(int((f.retryAfter+time.Second-1)/time.Second), 1)
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	slog.InfoContext(r.Context(), "dataentry: reconcile refused", "code", f.code)
	writeV1Error(w, r, f.status, f.code, f.title, "retry after "+strconv.Itoa(secs)+"s")
}
