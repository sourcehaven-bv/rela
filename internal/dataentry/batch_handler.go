package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// batchPath is POST /api/v1/_batch (TKT-BO8CYG9, DEC-ODU65KR).
const batchPath = "/api/v1/_batch"

// batchBodyLimit is the largest batch body read, 1 MiB. It is fixed
// (DEC-ODU65KR item 4), like [entitymanager.MaxBatchOperations].
const batchBodyLimit = 1 << 20

// batchHandler serves POST /api/v1/_batch: several entity creates and the
// relations between them, written in one store transaction that commits
// whole or not at all. It is a handler of its own because writeHandler is
// at its method cap.
//
// Every operation is checked as the caller, with the gates a single create
// runs: per entity the face, the id options and the create field gate, per
// relation the relation affordance verdict; the manager then runs the
// create and relation ACL, the read gate on existing ends and the schema
// checks before anything is written.
type batchHandler struct {
	schema func() *Schema
	// atomic is whether the store rolls a failed transaction back. The
	// endpoint answers 501 when it does not: a failure there would leave
	// part of the batch written.
	atomic bool
	// manager is the ungated manager: the field gate runs here, as on the
	// single create (validateCreateFieldWrite).
	manager     *entitymanager.Manager
	affordances affordanceService
	serializer  entitySerializer
	visible     visibleReader
	audit       func() audit.Audit
	provision   func(context.Context) context.Context
}

// newBatchHandler wires the batch handler from the app's collaborators.
func newBatchHandler(app *App, st store.Store, em *entitymanager.Manager) *batchHandler {
	return &batchHandler{
		schema:      app.State,
		atomic:      store.TxRollsBack(st),
		manager:     em,
		affordances: app.affordances,
		serializer:  app.serializer,
		visible:     app.visibleReader,
		audit:       func() audit.Audit { return app.auditSink },
		provision:   newProvisionSeam(app),
	}
}

// batchPlan is a decoded, shape-checked batch request.
type batchPlan struct {
	req        v1.BatchRequest
	candidates []*entityPkg.Entity
	batch      entitymanager.Batch
}

//archplan:auth=authz
func (h *batchHandler) handleV1Batch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", r.Method)
		return
	}
	if !h.atomic {
		writeV1Error(w, r, http.StatusNotImplemented, "atomic_batch_unsupported",
			"Atomic batch not supported",
			"this deployment's store cannot roll a failed write back; see capabilities.atomic_batch on /api/v1/_schema")
		return
	}
	if h.provision != nil {
		r = r.WithContext(h.provision(r.Context()))
	}
	var req v1.BatchRequest
	if !decodeBatchBody(w, r, &req) {
		return
	}
	s := h.schema()
	plan, ok := h.plan(w, r, s.Meta, req)
	if !ok || !h.gate(w, r, s.Meta, plan) {
		return
	}
	res, err := entitymanager.CreateBatch(r.Context(), h.manager, plan.batch)
	if err != nil {
		h.writeBatchError(w, r, plan, err)
		return
	}
	writeV1JSON(w, http.StatusOK, h.response(r.Context(), s.Meta, plan, res))
}

// decodeBatchBody reads at most [batchBodyLimit] bytes into dst, strictly:
// an unknown field or data after the JSON value is a 400.
func decodeBatchBody(w http.ResponseWriter, r *http.Request, dst *v1.BatchRequest) bool {
	data, err := io.ReadAll(io.LimitReader(r.Body, batchBodyLimit+1))
	switch {
	case err != nil:
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Could not read the request body", "")
		return false
	case len(data) > batchBodyLimit:
		writeV1Error(w, r, http.StatusRequestEntityTooLarge, "body_too_large", "Request body too large",
			"a batch body is at most 1 MiB")
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body",
			"unexpected data after the JSON object")
		return false
	}
	return true
}

// plan checks the shape of req and builds the engine batch: the operation
// count, the refs, each end naming exactly one entity, and per entity the
// type, face and id options. It writes the error itself and reports false
// when the request cannot run.
func (h *batchHandler) plan(
	w http.ResponseWriter, r *http.Request, meta *metamodel.Metamodel, req v1.BatchRequest,
) (*batchPlan, bool) {
	n := len(req.Entities) + len(req.Relations)
	if n == 0 || n > entitymanager.MaxBatchOperations {
		writeBatchProblem(w, r, http.StatusBadRequest, "batch_invalid", "Invalid batch",
			fmt.Sprintf("a batch holds 1 to %d entities and relations, not %d",
				entitymanager.MaxBatchOperations, n), "", nil)
		return nil, false
	}
	refs := make(map[string]int, len(req.Entities))
	plan := &batchPlan{
		req:        req,
		candidates: make([]*entityPkg.Entity, len(req.Entities)),
		batch: entitymanager.Batch{
			Entities:  make([]entitymanager.BatchEntity, len(req.Entities)),
			Relations: make([]entitymanager.BatchRelation, len(req.Relations)),
		},
	}
	for i, be := range req.Entities {
		if be.Ref != "" {
			if _, dup := refs[be.Ref]; dup {
				writeBatchProblem(w, r, http.StatusBadRequest, "batch_invalid", "Invalid batch",
					fmt.Sprintf("ref %q is defined twice", be.Ref), entityPointer(i, "ref"), entityOp(i, be.Ref))
				return nil, false
			}
			refs[be.Ref] = i
		}
		ent, ok := planEntity(w, r, meta, i, be)
		if !ok {
			return nil, false
		}
		plan.batch.Entities[i] = ent
		plan.candidates[i] = &entityPkg.Entity{Type: be.Type, Face: ent.Options.Face, Properties: be.Properties}
	}
	for j, br := range req.Relations {
		rel, ok := planRelation(w, r, refs, j, br)
		if !ok {
			return nil, false
		}
		plan.batch.Relations[j] = rel
	}
	return plan, true
}

// planEntity checks one entity of the request and returns its engine
// create. The wire carries no automation or external-ref option: those are
// trusted by the manager and never set from a request.
func planEntity(
	w http.ResponseWriter, r *http.Request, meta *metamodel.Metamodel, i int, be v1.BatchEntity,
) (entitymanager.BatchEntity, bool) {
	op := entityOp(i, be.Ref)
	def, ok := meta.Entities[be.Type]
	if !ok {
		writeBatchProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed",
			fmt.Sprintf("unknown entity type %q", be.Type), entityPointer(i, "type"), op)
		return entitymanager.BatchEntity{}, false
	}
	opts := entityPkg.CreateOptions{ID: strings.TrimSpace(be.ID), Prefix: strings.TrimSpace(be.Prefix)}
	if raw := strings.TrimSpace(be.Face); raw != "" {
		face, err := entityPkg.ParseFace(raw)
		if err != nil {
			writeBatchProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed",
				err.Error(), entityPointer(i, "face"), op)
			return entitymanager.BatchEntity{}, false
		}
		opts.Face = face
	}
	if msg := validateCreateIDOpts(&def, opts.ID, opts.Prefix); msg != "" {
		writeBatchProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed",
			msg, entityPointer(i, "id"), op)
		return entitymanager.BatchEntity{}, false
	}
	return entitymanager.BatchEntity{
		Entity:  &entityPkg.Entity{Type: be.Type, Properties: be.Properties, Content: be.Content},
		Options: opts,
	}, true
}

// planRelation resolves the ends of one relation of the request. A ref is
// only defined by an entity of the request, and every entity is written
// before any relation, so a ref is either defined or unknown.
func planRelation(
	w http.ResponseWriter, r *http.Request, refs map[string]int, j int, br v1.BatchRelation,
) (entitymanager.BatchRelation, bool) {
	from, ok := planEnd(w, r, refs, j, "from", br.From)
	if !ok {
		return entitymanager.BatchRelation{}, false
	}
	to, ok := planEnd(w, r, refs, j, "to", br.To)
	if !ok {
		return entitymanager.BatchRelation{}, false
	}
	if br.To.Face != "" {
		writeBatchProblem(w, r, http.StatusBadRequest, "batch_invalid", "Invalid batch",
			"a relation head names an entity, not a face", relationPointer(j, "to/face"), relationOp(j))
		return entitymanager.BatchRelation{}, false
	}
	return entitymanager.BatchRelation{From: from, Type: br.Type, To: to}, true
}

// planEnd resolves one end: a ref to the index of its entity, or an id.
func planEnd(
	w http.ResponseWriter, r *http.Request, refs map[string]int, j int, side string, end v1.BatchEnd,
) (entitymanager.BatchEnd, bool) {
	invalid := func(detail, pointer string) (entitymanager.BatchEnd, bool) {
		writeBatchProblem(w, r, http.StatusBadRequest, "batch_invalid", "Invalid batch",
			detail, relationPointer(j, pointer), relationOp(j))
		return entitymanager.BatchEnd{}, false
	}
	switch {
	case (end.Ref == "") == (end.ID == ""):
		return invalid("an end names exactly one of ref and id", side)
	case end.ID != "":
		addr, err := entityPkg.ParseAddress(strings.TrimSpace(end.ID))
		if err != nil {
			return invalid(err.Error(), side+"/id")
		}
		if _, named := addr.Named(); named {
			return invalid("an id names no face; put the face in face", side+"/id")
		}
		face, err := parseOptionalFace(end.Face)
		if err != nil {
			return invalid(err.Error(), side+"/face")
		}
		return entitymanager.BatchEnd{ID: addr.ID(), Face: face}, true
	case end.Face != "":
		return invalid("an end that is a ref leaves that entity's face", side+"/face")
	}
	i, ok := refs[end.Ref]
	if !ok {
		return invalid(fmt.Sprintf("ref %q names no entity of this batch", end.Ref), side+"/ref")
	}
	return entitymanager.BatchEnd{Index: i}, true
}

// parseOptionalFace parses a face from a request, the empty face included.
func parseOptionalFace(raw string) (entityPkg.Face, error) {
	if raw = strings.TrimSpace(raw); raw == "" {
		return "", nil
	}
	return entityPkg.ParseFace(raw)
}

// gate runs the gates a single create runs in the handler, before the
// manager: the create field gate per entity (BUG-Q60V) and the relation
// affordance verdict per relation. A denial is audited and answered 403.
func (h *batchHandler) gate(w http.ResponseWriter, r *http.Request, meta *metamodel.Metamodel, plan *batchPlan) bool {
	ctx := r.Context()
	for i, c := range plan.candidates {
		if denial := h.affordances.validateCreateFieldWrite(ctx, c, c.Properties); denial != nil {
			h.denyAfford(w, r, c, *denial, entityOp(i, plan.req.Entities[i].Ref))
			return false
		}
	}
	for j, br := range plan.batch.Relations {
		sources, err := h.relationSources(ctx, meta, plan, br)
		if err != nil {
			writeInternalError(w, r, "read_failed", "Failed to read relation source", err)
			return false
		}
		if row, denial := h.affordances.relationOpDenial(ctx, sources, br.Type, RelationOpCreate); denial != nil {
			h.denyAfford(w, r, row, *denial, relationOp(j))
			return false
		}
	}
	return true
}

// relationSources is what gates a create of br, as on a single create: a
// batch entity as the source is judged as its candidate; an existing source
// is read through the gated reader, with the edge's head as the path entity
// when it is a batch entity. An existing source the caller cannot read is
// judged like a missing one, and the manager refuses both alike.
func (h *batchHandler) relationSources(
	ctx context.Context, meta *metamodel.Metamodel, plan *batchPlan, br entitymanager.BatchRelation,
) ([]relationSource, error) {
	if br.From.ID == "" {
		return h.affordances.relationSources(ctx, plan.candidates[br.From.Index], entityPkg.Ref{}, "", br.Type)
	}
	var head *entityPkg.Entity
	if br.To.ID == "" {
		head = plan.candidates[br.To.Index]
	}
	tail := br.From.Face
	if !metamodel.IsContentScoped(meta, br.Type) {
		tail = ""
	}
	return h.affordances.relationSources(ctx, head, entityPkg.Ref{ID: br.From.ID, Face: tail},
		string(DirectionIncoming), br.Type)
}

// denyAfford audits an affordance denial and answers it with the
// operation it refused.
func (h *batchHandler) denyAfford(
	w http.ResponseWriter, r *http.Request, target *entityPkg.Entity, denial AffordanceDenialError,
	op *v1.ErrorOperation,
) {
	recordAffordanceDenial(r.Context(), h.audit(), target, denial)
	writeBatchV1(w, http.StatusForbidden, v1.Error{
		Type: "https://rela.dev/errors/forbidden", Title: "Forbidden", Status: http.StatusForbidden,
		Detail: denial.Reason, Instance: r.URL.Path, Operation: op,
		RuleKind: "affordance", RuleID: denial.RuleID(),
	})
}

// writeBatchError answers a failed [entitymanager.CreateBatch] with the
// operation it names, in the classes a single create uses.
func (h *batchHandler) writeBatchError(w http.ResponseWriter, r *http.Request, plan *batchPlan, err error) {
	if _, kept := errors.AsType[*entitymanager.BatchKeptError](err); kept {
		// Unreachable on a store that rolls back. Ids only, never the rows.
		writeInternalError(w, r, "batch_partially_written", "Batch partially written", err)
		return
	}
	var op *v1.ErrorOperation
	pointer := ""
	opErr, hasOp := errors.AsType[*entitymanager.BatchOpError](err)
	if hasOp {
		if opErr.Kind == entitymanager.BatchOpEntity {
			op = entityOp(opErr.Index, plan.req.Entities[opErr.Index].Ref)
			pointer = entityPointer(opErr.Index, "")
		} else {
			op = relationOp(opErr.Index)
			pointer = relationPointer(opErr.Index, "")
		}
	}
	if errors.Is(err, entitymanager.ErrBatchInvalid) {
		writeBatchProblem(w, r, http.StatusBadRequest, "batch_invalid", "Invalid batch", err.Error(), pointer, op)
		return
	}
	switch entitymanager.ClassifyCreateError(err) {
	case entitymanager.CreateErrForbidden:
		if writeBatchForbidden(w, r, err, op) {
			return
		}
	case entitymanager.CreateErrFace:
		writeBatchProblem(w, r, http.StatusUnprocessableEntity, "face_required", "Face required",
			err.Error(), pointer, op)
		return
	case entitymanager.CreateErrConflict:
		writeBatchProblem(w, r, http.StatusConflict, "conflict", "Conflict", err.Error(), pointer, op)
		return
	case entitymanager.CreateErrInvalid:
		writeBatchInvalid(w, r, err, opErr, pointer, op)
		return
	case entitymanager.CreateErrUnknown:
		// A store or gate fault, not a refusal: its text can carry driver
		// detail, and a client should retry it rather than fix the request.
		if !writeNoIdentity(w, r, err) {
			writeInternalError(w, r, "batch_failed", "Batch failed", err)
		}
		return
	}
	writeBatchProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed",
		err.Error(), pointer, op)
}

// writeBatchInvalid answers a refused operation: a missing or hidden end
// as target_not_found at that end, both alike; a type-disallowed edge with
// its code; anything else as validation_failed.
func writeBatchInvalid(
	w http.ResponseWriter, r *http.Request, err error, opErr *entitymanager.BatchOpError, pointer string,
	op *v1.ErrorOperation,
) {
	if opErr != nil && opErr.End != "" && errors.Is(err, entitymanager.ErrEntityNotFound) {
		writeBatchProblem(w, r, http.StatusUnprocessableEntity, "target_not_found", "Relation end not found",
			err.Error(), relationPointer(opErr.Index, opErr.End), op)
		return
	}
	if invalid, ok := errors.AsType[*entitymanager.InvalidRelationError](err); ok {
		if code := invalid.TypeNotAllowedCode(); code != "" {
			writeBatchProblem(w, r, http.StatusUnprocessableEntity, code, "Validation failed",
				err.Error(), pointer, op)
			return
		}
	}
	writeBatchProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed",
		err.Error(), pointer, op)
}

// writeBatchForbidden answers an ACL refusal of one operation, after the
// identity faults that are not refusals. It reports whether it wrote.
func writeBatchForbidden(w http.ResponseWriter, r *http.Request, err error, op *v1.ErrorOperation) bool {
	fe, ok := errors.AsType[*acl.ForbiddenError](err)
	if !ok {
		return false
	}
	if writeIdentityFaultDeny(w, r, fe) {
		return true
	}
	writeBatchV1(w, http.StatusForbidden, v1.Error{
		Type: "https://rela.dev/errors/forbidden", Title: "Forbidden", Status: http.StatusForbidden,
		Detail: fe.Decision.Reason, Instance: r.URL.Path, Operation: op,
		RuleKind: fe.Decision.RuleKind, RuleID: fe.Decision.RuleID,
	})
	return true
}

// response is the 200 body: per entity its ref and id, and the row as the
// caller may read it.
func (h *batchHandler) response(
	ctx context.Context, meta *metamodel.Metamodel, plan *batchPlan, res *entitymanager.BatchResult,
) v1.BatchResponse {
	created := make([]*entityPkg.Entity, len(res.Entities))
	for i, c := range res.Entities {
		created[i] = c.Entity
	}
	readable := make(map[entityPkg.Ref]bool, len(created))
	for _, e := range h.visible.filterVisible(ctx, created) {
		readable[e.Ref()] = true
	}
	out := v1.BatchResponse{
		Entities:  make([]v1.BatchCreatedEntity, len(res.Entities)),
		Relations: make([]v1.BatchCreatedRelation, len(res.Relations)),
	}
	for i, c := range res.Entities {
		e := c.Entity
		item := v1.BatchCreatedEntity{Index: i, Ref: plan.req.Entities[i].Ref, ID: e.ID, Face: e.Face.String()}
		if readable[e.Ref()] {
			// Warnings are computed after the automations ran, so their text
			// is only shown with the row it is about.
			item.Warnings = c.Warnings
			wire := h.serializer.forWireRelated(ctx, e, nil, nil, nil, meta, meta.Entities[e.Type].Plural)
			item.Entity = &wire
		}
		if fErr := res.FollowUpErrs[i]; fErr != nil {
			// The text can name rows the caller cannot read; it goes to the log.
			slog.WarnContext(ctx, "dataentry: batch: a step after a create failed",
				"entity", e.ID, "error", fErr)
			item.FollowUpError = "follow_up_failed"
		}
		out.Entities[i] = item
	}
	for j, rel := range res.Relations {
		out.Relations[j] = v1.BatchCreatedRelation{
			Index: j, From: rel.From, FromFace: rel.FromFace.String(), Type: rel.Type, To: rel.To,
		}
	}
	return out
}

// writeBatchProblem writes a problem+json error about one operation, with
// pointer into the request body when it names a field.
func writeBatchProblem(
	w http.ResponseWriter, r *http.Request, status int, code, title, detail, pointer string,
	op *v1.ErrorOperation,
) {
	e := v1.Error{
		Type: "https://rela.dev/errors/" + code, Title: title, Status: status,
		Detail: detail, Instance: r.URL.Path, Operation: op,
	}
	if pointer != "" {
		e.Errors = []v1.FieldError{{Source: v1.ErrorSource{Pointer: pointer}, Code: code, Detail: detail}}
	}
	writeBatchV1(w, status, e)
}

// writeBatchV1 writes e as problem+json.
func writeBatchV1(w http.ResponseWriter, status int, e v1.Error) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(e)
}

func entityOp(i int, ref string) *v1.ErrorOperation {
	return &v1.ErrorOperation{Kind: entitymanager.BatchOpEntity, Index: i, Ref: ref}
}

func relationOp(j int) *v1.ErrorOperation {
	return &v1.ErrorOperation{Kind: entitymanager.BatchOpRelation, Index: j}
}

// entityPointer is the JSON pointer of entity i, or of its field.
func entityPointer(i int, field string) string {
	return opPointer("entities", i, field)
}

// relationPointer is the JSON pointer of relation j, or of a path in it.
func relationPointer(j int, path string) string {
	return opPointer("relations", j, path)
}

func opPointer(list string, i int, path string) string {
	p := "/" + list + "/" + strconv.Itoa(i)
	if path != "" {
		p += "/" + path
	}
	return p
}
