package dataentry

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowview"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const (
	flowsPath = "/api/v1/_flows"
	// flowBodyLimit caps a flow request body: a step's property values and
	// a start's flow name and subject address.
	flowBodyLimit = 256 << 10
)

// FlowService is the flow capability the flow routes use. The wiring site
// passes the project's *appbuild.FlowControl, which reads through the
// caller's row gate and checks the flow's roles and permissions itself.
// Its errors carry a [flowview] kind, except write errors from the step's
// update, which keep the write path's own shape.
type FlowService interface {
	Flows() []flowview.Flow
	RunList(ctx context.Context, flowName, status string, page, perPage int) (flowview.Runs, error)
	SubjectRuns(ctx context.Context, addr string) ([]flowview.Run, error)
	Run(ctx context.Context, id flow.RunID) (flowview.Run, error)
	Start(ctx context.Context, name, addr string) (flow.RunID, error)
	Complete(ctx context.Context, id flow.RunID, gen int64, props map[string]any) (flowview.Completed, error)
	Cancel(ctx context.Context, id flow.RunID) error
	Retry(ctx context.Context, id flow.RunID) error
	Tasks(ctx context.Context, flowName string, page, perPage int) (flowview.Tasks, error)
}

// flowsHandler owns the flow routes (TKT-ZAXRXND):
//
//	GET  /api/v1/_flows                       the flows of flows.yaml
//	GET  /api/v1/_flows/tasks                 the caller's tasks
//	GET  /api/v1/_flows/runs                  the run list; ?subject= the runs on one entity
//	POST /api/v1/_flows/runs                  start a run (201)
//	GET  /api/v1/_flows/runs/{id}             one run, with why it waits and `_actions`
//	POST /api/v1/_flows/runs/{id}/_complete   complete the step
//	POST /api/v1/_flows/runs/{id}/_cancel     cancel (204)
//	POST /api/v1/_flows/runs/{id}/_retry      retry the pending outputs (204)
//
// A nil *flowsHandler is "no flows declared": every route answers 404.
// [SetFlows] installs it.
//
// A run the caller cannot read is the uniform 404, like a hidden entity.
// A missing role or permission is a 403 that names it: flows.yaml is
// configuration, not a secret.
type flowsHandler struct {
	svc FlowService
}

// SetFlows installs the flow service. Call it before [App.NewRouter]. It
// is a package function because App is at its plimsoll method cap.
//
// Nil: rejected — a project without flows never calls this, so a nil here
// is a wiring bug.
func SetFlows(a *App, svc FlowService) error {
	if svc == nil {
		return errors.New("dataentry: SetFlows requires a flow service")
	}
	a.flows = &flowsHandler{svc: svc}
	return nil
}

// flowStartRequest is the body of POST /_flows/runs.
type flowStartRequest struct {
	Flow    string `json:"flow"`
	Subject string `json:"subject"`
}

// flowCompleteRequest is the body of POST /_flows/runs/{id}/_complete.
type flowCompleteRequest struct {
	// Generation is the run generation the form was rendered at.
	Generation int64          `json:"generation"`
	Properties map[string]any `json:"properties"`
}

// flowStarted answers a start.
type flowStarted struct {
	ID string `json:"id"`
}

// flowList answers GET /_flows.
type flowList struct {
	Data []flowview.Flow `json:"data"`
}

// handleV1Flows serves every route under /api/v1/_flows.
//
//archplan:auth=authz reason="reads through the row gate; steps check the flow's roles, then re-authorize the write"
func (h *flowsHandler) handleV1Flows(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, flowsPath), "/"), "/")
	switch {
	case len(parts) == 1 && parts[0] == "":
		if allowMethod(w, r, http.MethodGet) {
			writeV1JSON(w, http.StatusOK, flowList{Data: orEmpty(h.svc.Flows())})
		}
	case len(parts) == 1 && parts[0] == "tasks":
		if allowMethod(w, r, http.MethodGet) {
			h.tasks(w, r)
		}
	case len(parts) == 1 && parts[0] == "runs":
		switch r.Method {
		case http.MethodGet:
			h.runs(w, r)
		case http.MethodPost:
			h.start(w, r)
		default:
			w.Header().Set("Allow", "GET, POST")
			writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		}
	case len(parts) == 2 && parts[0] == "runs" && parts[1] != "":
		if allowMethod(w, r, http.MethodGet) {
			h.run(w, r, flow.RunID(parts[1]))
		}
	case len(parts) == 3 && parts[0] == "runs" && parts[1] != "":
		if allowMethod(w, r, http.MethodPost) {
			h.act(w, r, flow.RunID(parts[1]), parts[2])
		}
	default:
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
	}
}

func (h *flowsHandler) tasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, perPage := parseV1Pagination(q)
	out, err := h.svc.Tasks(r.Context(), q.Get("flow"), page, perPage)
	if err != nil {
		writeFlowError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, out)
}

// runs answers the run list, or with ?subject= every run on that entity
// the caller can see.
func (h *flowsHandler) runs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if addr := q.Get("subject"); addr != "" {
		runs, err := h.svc.SubjectRuns(r.Context(), addr)
		if err != nil {
			writeFlowError(w, r, err)
			return
		}
		writeV1JSON(w, http.StatusOK, flowview.Runs{Runs: orEmpty(runs), Total: len(runs)})
		return
	}
	page, perPage := parseV1Pagination(q)
	out, err := h.svc.RunList(r.Context(), q.Get("flow"), q.Get("status"), page, perPage)
	if err != nil {
		writeFlowError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, out)
}

func (h *flowsHandler) start(w http.ResponseWriter, r *http.Request) {
	var req flowStartRequest
	if !decodeStrictBody(w, r, flowBodyLimit, "256 KiB", &req) {
		return
	}
	if req.Flow == "" || req.Subject == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "flow and subject are required", "")
		return
	}
	id, err := h.svc.Start(r.Context(), req.Flow, req.Subject)
	if err != nil {
		writeFlowError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusCreated, flowStarted{ID: string(id)})
}

func (h *flowsHandler) run(w http.ResponseWriter, r *http.Request, id flow.RunID) {
	out, err := h.svc.Run(r.Context(), id)
	if err != nil {
		writeFlowError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, out)
}

// act serves the POSTs on one run.
func (h *flowsHandler) act(w http.ResponseWriter, r *http.Request, id flow.RunID, op string) {
	switch op {
	case "_complete":
		var req flowCompleteRequest
		if !decodeStrictBody(w, r, flowBodyLimit, "256 KiB", &req) {
			return
		}
		out, err := h.svc.Complete(r.Context(), id, req.Generation, req.Properties)
		if err != nil {
			writeFlowError(w, r, err)
			return
		}
		if out.Written == nil {
			out.Written = []string{}
		}
		writeV1JSON(w, http.StatusOK, out)
	case "_cancel", "_retry":
		var err error
		if op == "_cancel" {
			err = h.svc.Cancel(r.Context(), id)
		} else {
			err = h.svc.Retry(r.Context(), id)
		}
		if err != nil {
			writeFlowError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
	}
}

// writeFlowError maps a flow error to its status. A hidden run is the
// entity GET's uniform 404 with no detail. A refusal of the step's write
// keeps the write path's own answer (403 for the ACL, 422 for validation,
// 409 for a lost race); anything else is a 500 with the cause in the log.
func writeFlowError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case requestCanceled(r, err):
		return
	case writeNoIdentity(w, r, err):
		return
	case errors.Is(err, flowview.ErrNotFound):
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
	case errors.Is(err, flowview.ErrConflict):
		writeV1Error(w, r, http.StatusConflict, "flow_conflict", "The run cannot do that now", err.Error())
	case errors.Is(err, flowview.ErrForbidden):
		writeV1Error(w, r, http.StatusForbidden, "forbidden", "Permission denied", err.Error())
	case errors.Is(err, flowview.ErrInvalid):
		writeV1Error(w, r, http.StatusUnprocessableEntity, "flow_invalid", "The flow does not allow that",
			err.Error())
	case errors.Is(err, entitymanager.ErrEntityNotFound):
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
	case writeForbiddenIfACLDenied(w, r, err):
		return
	default:
		if _, ok := errors.AsType[*store.VersionConflictError](err); ok {
			writeV1Error(w, r, http.StatusConflict, "conflict", "Entity is being modified concurrently",
				"retry the request")
			return
		}
		if _, ok := errors.AsType[*entitymanager.ValidationError](err); ok {
			writeWriteRefused(w, r, err)
			return
		}
		writeInternalError(w, r, "flow_failed", "Flow request failed", err)
	}
}

// orEmpty returns s, or an empty slice for nil, so a list encodes as [].
func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
