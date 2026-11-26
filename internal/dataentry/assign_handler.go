package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// assignPath serves list and page `assign:` blocks (TKT-FGE5G3I).
//
// GET  ?list=ID or ?page=ID&target=ID answers the resolved values, which the
// SPA uses as a create form's prefill.
// POST {list|page, target, entities} writes them to each entity and answers
// per entity whether the write landed and, for a list, whether the entity now
// belongs to the list.
const assignPath = "/api/v1/_assign"

// maxAssignEntities caps one POST. A drop or a keyboard move names the
// selected rows; a larger batch is a script's job.
const maxAssignEntities = 100

// maxAssignBody caps the POST body: 100 addresses plus the keys.
const maxAssignBody = 64 << 10

type assignRequest struct {
	List     string   `json:"list,omitempty"`
	Page     string   `json:"page,omitempty"`
	Target   string   `json:"target,omitempty"`
	Entities []string `json:"entities,omitempty"`
}

// assignValues is the resolved block: the GET answer and the PATCH body of
// every POST row.
type assignValues struct {
	EntityType string                       `json:"entity_type"`
	Properties map[string]any               `json:"properties,omitempty"`
	Relations  map[string]assignRelationOps `json:"relations,omitempty"`
}

// assignRelationOps is one relation in the PATCH wire shape: `data` replaces
// the edges of a relation that allows one outgoing edge, `add` adds to any
// other, so an assign never removes edges of a many-relation.
type assignRelationOps struct {
	Data []assignPeer `json:"data,omitempty"`
	Add  []assignPeer `json:"add,omitempty"`
}

type assignPeer struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type assignResult struct {
	ID      string `json:"id"`
	OK      bool   `json:"ok"`
	Status  int    `json:"status,omitempty"`
	Code    string `json:"code,omitempty"`
	Matches *bool  `json:"matches,omitempty"`
}

type assignResponse struct {
	Results []assignResult `json:"results"`
}

// errAssignTargetNotFound is a target the caller cannot read, answered as
// the uniform 404 of a missing entity.
var errAssignTargetNotFound = errors.New("assign target not found")

func handleV1Assign(a *App, w http.ResponseWriter, r *http.Request) {
	var req assignRequest
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		req = assignRequest{List: q.Get("list"), Page: q.Get("page"), Target: q.Get("target")}
	case http.MethodPost:
		body, err := io.ReadAll(io.LimitReader(r.Body, maxAssignBody+1))
		if err != nil || len(body) > maxAssignBody {
			writeV1Error(w, r, http.StatusRequestEntityTooLarge, "body_too_large", "Request body too large", "")
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
			return
		}
		if len(req.Entities) == 0 || len(req.Entities) > maxAssignEntities {
			writeV1Error(w, r, http.StatusBadRequest, "bad_request",
				fmt.Sprintf("entities must name 1 to %d entities", maxAssignEntities), "")
			return
		}
	default:
		w.Header().Set("Allow", "GET, POST")
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	s := a.State()
	vals, status, code, err := resolveAssign(r.Context(), a, s, req, time.Now())
	if err != nil {
		if status >= http.StatusInternalServerError {
			writeInternalError(w, r, code, "Assign failed", err)
			return
		}
		writeV1Error(w, r, status, code, err.Error(), "")
		return
	}
	if r.Method == http.MethodGet {
		writeV1JSON(w, http.StatusOK, vals)
		return
	}
	results, err := applyAssign(a, r, s, vals, req.Entities)
	if err != nil {
		writeInternalError(w, r, "assign_failed", "Assign failed", err)
		return
	}
	if req.List != "" {
		if err := markAssignMatches(r.Context(), a, s, req.List, results); err != nil {
			writeListPipelineError(w, r, err)
			return
		}
	}
	writeV1JSON(w, http.StatusOK, assignResponse{Results: results})
}

// assignFailure is a refused or failed resolve. An internal one answers 500
// through writeInternalError, so its error text reaches only the log.
type assignFailure struct {
	status   int
	internal bool
	code     string
	err      error
}

func assignInternal(code string, err error) *assignFailure {
	return &assignFailure{internal: true, code: code, err: err}
}

// resolveAssign finds the block the request names and evaluates it.
func resolveAssign(
	ctx context.Context, a *App, s *Schema, req assignRequest, now time.Time,
) (vals assignValues, status int, code string, err error) {
	block, itemType, targetType, fail := assignBlock(s, req)
	if fail == nil {
		vals, fail = assignValuesOf(ctx, a, s, block, itemType, targetType, req.Target, now)
		if fail == nil {
			return vals, 0, "", nil
		}
	}
	if fail.internal {
		return assignValues{}, http.StatusInternalServerError, fail.code, fail.err
	}
	return assignValues{}, fail.status, fail.code, fail.err
}

// assignBlock finds the block. A missing list or page is a 404 naming the
// config key: config is not secret.
func assignBlock(
	s *Schema, req assignRequest,
) (block *dataentryconfig.Assign, itemType, targetType string, fail *assignFailure) {
	switch {
	case req.List != "" && req.Page == "":
		l, ok := s.Cfg.Lists[req.List]
		if !ok || l.Assign.IsZero() {
			return nil, "", "", &assignFailure{status: http.StatusNotFound, code: "assign_not_found",
				err: fmt.Errorf("list %q has no assign", req.List)}
		}
		return l.Assign, l.EntityType, "", nil
	case req.Page != "" && req.List == "":
		p, ok := s.Cfg.Pages[req.Page]
		if !ok || p.Assign.IsZero() {
			return nil, "", "", &assignFailure{status: http.StatusNotFound, code: "assign_not_found",
				err: fmt.Errorf("page %q has no assign", req.Page)}
		}
		return p.Assign, p.Assign.EntityType, p.EntityType, nil
	default:
		return nil, "", "", &assignFailure{status: http.StatusBadRequest, code: "bad_request",
			err: errors.New("name exactly one of list and page")}
	}
}

func assignValuesOf(
	ctx context.Context, a *App,
	s *Schema, block *dataentryconfig.Assign, itemType, targetType, target string, now time.Time,
) (assignValues, *assignFailure) {
	vals := assignValues{EntityType: itemType, Properties: map[string]any{}}
	entDef, _ := s.Meta.GetEntityDef(itemType)
	for name, v := range block.Properties {
		got, err := assignProperty(ctx, a, s, entDef.Properties[name], v, now)
		if err != nil {
			return assignValues{}, assignInternal("assign_failed", err)
		}
		vals.Properties[name] = got
	}
	if len(block.Relations) > 0 {
		vals.Relations = map[string]assignRelationOps{}
	}
	for rel, val := range block.Relations {
		peer, fail := assignPeerOf(ctx, a, val, targetType, target)
		if fail != nil {
			return assignValues{}, fail
		}
		relDef, _ := s.Meta.GetRelationDef(rel)
		if relDef != nil && relDef.MaxOutgoing != nil && *relDef.MaxOutgoing == 1 {
			vals.Relations[rel] = assignRelationOps{Data: []assignPeer{peer}}
		} else {
			vals.Relations[rel] = assignRelationOps{Add: []assignPeer{peer}}
		}
	}
	return vals, nil
}

func assignProperty(
	ctx context.Context, a *App,
	s *Schema, def metamodel.PropertyDef, v dataentryconfig.AssignValue, now time.Time,
) (any, error) {
	if v.Expr == "" {
		return dataentryconfig.TypedLiteral(&def, v.Literal)
	}
	if a.assignExprs == nil {
		return nil, errors.New("assign expressions are not wired")
	}
	return a.assignExprs(ctx, s.Meta, &def, v.Expr, now)
}

// assignPeerOf resolves a relation value: the request's target, or a
// configured entity id. Either one the caller cannot read is the uniform 404.
func assignPeerOf(ctx context.Context, a *App, val, targetType, target string) (assignPeer, *assignFailure) {
	notFound := &assignFailure{status: http.StatusNotFound, code: "not_found", err: errAssignTargetNotFound}
	if val != dataentryconfig.AssignTarget {
		typ, err := a.visibleReader.readableType(ctx, val)
		if err != nil || typ == "" {
			return assignPeer{}, notFound
		}
		return assignPeer{ID: val, Type: typ}, nil
	}
	if target == "" {
		return assignPeer{}, &assignFailure{status: http.StatusBadRequest, code: "bad_request",
			err: errors.New("target is required")}
	}
	got, found, err := a.visibleReader.inWorld(ctx, targetType, target)
	if err != nil {
		return assignPeer{}, assignInternal("read_failed", err)
	}
	if !found {
		return assignPeer{}, notFound
	}
	return assignPeer{ID: got.ID, Type: got.Type}, nil
}

// AssignExprFunc evaluates an `assign:` value expression at now and returns
// the value in the property's wire form, nil for unset.
//
// Consumer-side, like [ViewConditionFunc]: the predicate engine sits above
// this package, so the composition root supplies conditionlint.EvalAssignExpr.
type AssignExprFunc func(
	ctx context.Context, meta *metamodel.Metamodel, def *metamodel.PropertyDef, expr string, now time.Time,
) (any, error)

// SetAssignExprs injects the evaluator of `assign:` expressions.
//
// Nil: rejected. Without it a list assign with an expression would answer
// 500 on every drop, so a wiring that forgets it fails at startup.
func SetAssignExprs(a *App, fn AssignExprFunc) error {
	if fn == nil {
		return errors.New("dataentry.SetAssignExprs: func must be non-nil")
	}
	a.assignExprs = fn
	return nil
}

// applyAssign writes vals to each address through the entity PATCH handler,
// so an assign is authorized, validated and audited exactly as a PATCH from
// the SPA is. Each row is its own write: a refusal leaves the others written.
func applyAssign(a *App, r *http.Request, s *Schema, vals assignValues, addrs []string) ([]assignResult, error) {
	plural := pluralOf(s.Meta)(vals.EntityType)
	patch := map[string]any{}
	var unset []string
	props := map[string]any{}
	for k, v := range vals.Properties {
		if v == nil {
			unset = append(unset, k)
			continue
		}
		props[k] = v
	}
	if len(props) > 0 {
		patch["properties"] = props
	}
	if len(unset) > 0 {
		patch["properties_unset"] = unset
	}
	if len(vals.Relations) > 0 {
		patch["relations"] = vals.Relations
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}
	results := make([]assignResult, 0, len(addrs))
	seen := map[string]bool{}
	for _, addr := range addrs {
		if seen[addr] {
			continue
		}
		seen[addr] = true
		res := assignResult{ID: addr}
		sub := r.Clone(r.Context())
		sub.Method = http.MethodPatch
		sub.Body = io.NopCloser(bytes.NewReader(body))
		sub.ContentLength = int64(len(body))
		sub.Header.Del("If-Match")
		rec := httptest.NewRecorder()
		//nolint:contextcheck // sub carries the request's own ctx, cloned above
		a.write.handleV1UpdateEntity(rec, sub, vals.EntityType, plural, addr)
		res.Status = rec.Code
		res.OK = rec.Code < http.StatusMultipleChoices
		if !res.OK {
			res.Code = errorCodeOf(rec.Body.Bytes())
		}
		if res.OK {
			if id, _, err := entityPkg.ParseStateRef(addr); err == nil {
				res.ID = id
			}
		}
		results = append(results, res)
	}
	return results, nil
}

// markAssignMatches sets Matches on every written row: whether the list holds
// it now. One list read narrowed to the written ids, through the same pipeline
// as the list itself, so the answer covers only rows the caller can read.
func markAssignMatches(ctx context.Context, a *App, s *Schema, listID string, results []assignResult) error {
	reach := map[string]bool{}
	for _, r := range results {
		if r.OK {
			reach[r.ID] = true
		}
	}
	if len(reach) == 0 {
		return nil
	}
	listCfg := s.Cfg.Lists[listID]
	_, rows, err := configuredListRows(ctx, a, s, listID, listCfg, navStatusListQuery(listCfg), reach)
	if err != nil {
		return err
	}
	in := map[string]bool{}
	for _, e := range rows {
		in[e.ID] = true
	}
	for i := range results {
		if results[i].OK {
			m := in[results[i].ID]
			results[i].Matches = &m
		}
	}
	return nil
}

// errorCodeOf reads the code of a PATCH error body: the last segment of a
// problem `type` URL, the `errors[]` envelope, or the flat `{"error": ...}`
// of an ACL denial.
func errorCodeOf(body []byte) string {
	var e struct {
		Type   string `json:"type"`
		Error  string `json:"error"`
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if json.Unmarshal(body, &e) != nil {
		return ""
	}
	switch {
	case len(e.Errors) > 0:
		return e.Errors[0].Code
	case e.Type != "":
		return path.Base(e.Type)
	default:
		return e.Error
	}
}
