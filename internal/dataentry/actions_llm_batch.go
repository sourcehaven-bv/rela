package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/actionproposal"
	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/llmtask"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// llmBatchTimeout caps the AI call of a batch run. A reply for many rows
// takes longer than one for a single row, so it gets more time than
// [llmChatTimeout]. A proxy in front of the server must allow as much.
const llmBatchTimeout = 5 * time.Minute

// checkBatchPayload refuses a body whose shape does not fit the action: a
// list of addresses for an action without a batch block, both entity_id and
// entity_ids, an empty list, or more addresses than batch.max allows.
func checkBatchPayload(id string, action dataentryconfig.Action, payload actionPayload) *opFailure {
	if !payload.Batch {
		return nil
	}
	if action.LLM == nil || action.LLM.Batch == nil {
		return failV1(http.StatusBadRequest, "batch_not_supported",
			fmt.Sprintf("action %q does not run on several entities at once", id))
	}
	if payload.EntityID != "" {
		return failV1(http.StatusBadRequest, "invalid_body", "Send entity_id or entity_ids, not both")
	}
	if len(payload.EntityIDs) == 0 {
		return failV1(http.StatusBadRequest, "invalid_body", "entity_ids is empty")
	}
	if n, max := len(payload.EntityIDs), action.LLM.Batch.EffectiveMax(); n > max {
		return failV1(http.StatusBadRequest, "batch_too_large",
			fmt.Sprintf("action %q runs on at most %d entities at once; %d were sent", id, max, n))
	}
	return nil
}

// llmBatchRow is one row of a batch run: the stored row, used only to ask
// what the caller may write, and the row as the caller may see it.
type llmBatchRow struct {
	raw, redacted *entityPkg.Entity
}

// runLLMBatch runs a batch `llm:` action on the entities at addresses
// (TKT-I2SRNWR). Each address is gated as the detail action is: the caller
// must read the row, and the row must match available_on and when. A row
// that fails, or on which the caller may write none of the outputs, is
// skipped and counted. The rows left go to the model in one call. Like
// [actionOps.runLLM] it writes nothing.
func (o actionOps) runLLMBatch(
	ctx context.Context, s *Schema, id string, action dataentryconfig.Action,
	addresses []string, deps lua.WriteDeps,
) (*script.ActionResponse, *opFailure) {
	task, err := llmtask.New(id, action, s.Meta)
	if err != nil {
		return nil, failLLMInternal(err)
	}
	gated, sent, err := o.batchRows(ctx, s, id, action, addresses)
	if err != nil {
		return nil, failGate(err)
	}
	rows := make([]llmtask.BatchRow, 0, len(gated))
	for _, g := range gated {
		writable := o.llmWritable(ctx, g.raw, task.Config().Outputs.Set)
		if len(writable) == 0 {
			continue
		}
		rows = append(rows, llmtask.BatchRow{Subject: g.redacted, Writable: writable})
	}
	out := actionproposal.NewBatch()
	out.Skipped = sent - len(rows)
	if len(rows) == 0 {
		return &script.ActionResponse{Batch: out}, nil
	}

	provider, err := script.AIProvider(deps)
	if errors.Is(err, ai.ErrConfigNotFound) {
		return nil, failV1(http.StatusServiceUnavailable, "ai_not_configured",
			"No AI provider is configured for this project")
	}
	if err != nil {
		f := failV1(http.StatusInternalServerError, "ai_config_invalid", "The AI configuration could not be loaded")
		f.cause = err
		return nil, f
	}
	req, err := task.BatchRequest(rows)
	if errors.Is(err, llmtask.ErrBatchTooLarge) {
		return nil, failV1(http.StatusUnprocessableEntity, "batch_too_large", err.Error())
	}
	if err != nil {
		return nil, failLLMInternal(err)
	}
	callCtx, cancel := context.WithTimeout(ctx, llmBatchTimeout)
	defer cancel()
	resp, err := provider.Chat(callCtx, req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, failLLMInternal(ctxErr)
		}
		kind, status := ai.ErrKind(""), 0
		if aiErr := (*ai.Error)(nil); errors.As(err, &aiErr) {
			kind, status = aiErr.Kind, aiErr.Status
		}
		slog.Warn("llm action: AI call failed", "action", id, "rows", len(rows), "kind", kind, "status", status)
		return nil, failV1(http.StatusBadGateway, "ai_failed", llmCallDetail(err))
	}
	parsed, err := task.ParseBatch(resp, rows)
	if err != nil {
		var re *llmtask.ReplyError
		if errors.As(err, &re) {
			slog.Warn("llm action: unusable AI reply", "action", id, "rows", len(rows))
			return nil, failV1(http.StatusBadGateway, "ai_reply_invalid", re.Error())
		}
		return nil, failLLMInternal(err)
	}
	// One line per call that sent entity data to the provider. Not the
	// ids: the log is read by more people than the data.
	slog.Info("llm action: batch run", "action", id, "rows", len(rows),
		"failed", len(parsed.Failed), "model", resp.Model)
	parsed.Skipped = out.Skipped
	return &script.ActionResponse{Batch: parsed}, nil
}

// batchRows gates the addresses of a batch run in one pass, and returns the
// rows the action may run on, in the order sent, with the number of distinct
// addresses sent.
//
// The reads are batched like a list page: one header read resolves every
// address to the face the caller's world serves, one read per face loads the
// rows, the loaded rows are gated again per type, and each type's when is
// judged in one [ViewConditionMatcher.MatchPage] over the redacted rows. Field verdicts are per row, as on a list page.
//
// An address that is malformed, unreadable, names a face other than the one
// served, or fails available_on or when, is left out. They are not told
// apart: the caller learns only how many rows ran.
func (o actionOps) batchRows(
	ctx context.Context, s *Schema, id string, action dataentryconfig.Action, addresses []string,
) ([]llmBatchRow, int, error) {
	refs := make([]entityPkg.Ref, 0, len(addresses))
	ids := make([]string, 0, len(addresses))
	sent := 0
	seen := map[string]bool{}
	for _, a := range addresses {
		if seen[a] {
			continue
		}
		seen[a] = true
		sent++
		ref, err := entityPkg.ParseRef(a)
		if err != nil {
			continue
		}
		refs = append(refs, ref)
		ids = append(ids, ref.ID)
	}
	served, err := o.h.visible.servedIDsErr(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	keys := make([]entityPkg.Ref, 0, len(refs))
	taken := map[entityPkg.Ref]bool{}
	for _, ref := range refs {
		hdr, ok := served[ref.ID]
		if !ok || (ref.Face != "" && ref.Face != hdr.Face) {
			// A face other than the served one is skipped. llm actions
			// run only on types without faces (validateLLMAction), so
			// no readable row is lost here; revisit with face support.
			continue
		}
		key := entityPkg.Ref{ID: hdr.ID, Face: hdr.Face}
		if taken[key] {
			// Two addresses for one row, such as a bare id and its
			// served face: the row runs once.
			sent--
			continue
		}
		taken[key] = true
		keys = append(keys, key)
	}
	loaded, err := loadRows(ctx, o.h.store, keys)
	if err != nil {
		return nil, 0, err
	}
	raws := make([]*entityPkg.Entity, 0, len(keys))
	for _, k := range keys {
		if e, ok := loaded[k]; ok {
			raws = append(raws, e)
		}
	}
	// The header read above gated each id; the rows were read again
	// afterwards. Gate what was read, so a row that became unreadable in
	// between is not sent. One probe per type.
	raws = o.h.visible.filterVisible(ctx, raws)
	return batchActionGate{svc: o.h.affordances}.rows(ctx, s, id, action, raws), sent, nil
}

// batchActionGate is [detailActionCheck.Allows] for many rows of one action
// at once. It is its own type so affordanceService stays under its method
// load line.
type batchActionGate struct {
	svc affordanceService
}

// rows returns the rows in raws the action may run on, each with its
// redacted form. The caller has checked the permission and row-gated raws.
// The conditions compile once, and each type's when runs as one page.
//
//archplan:guard=authz reason="batch detail action gate: available_on and when: on the redacted rows"
func (g batchActionGate) rows(
	ctx context.Context, s *Schema, id string, action dataentryconfig.Action, raws []*entityPkg.Entity,
) []llmBatchRow {
	svc := g.svc
	rows := make([]llmBatchRow, 0, len(raws))
	hidden := make([]map[string]struct{}, 0, len(raws))
	for _, e := range raws {
		if !actionScopeAllows(action, e) {
			continue
		}
		h := svc.hiddenProperties(ctx, e)
		rows = append(rows, llmBatchRow{raw: e, redacted: visibility.Redact(ctx, fixedRedactor(h), e)})
		hidden = append(hidden, h)
	}
	if action.When == "" || len(rows) == 0 {
		return rows
	}
	lookup := svc.actionConditionLookup(s, id)
	byType := map[string][]int{}
	for i, r := range rows {
		byType[r.raw.Type] = append(byType[r.raw.Type], i)
	}
	keep := make([]bool, len(rows))
	for _, typ := range slices.Sorted(maps.Keys(byType)) {
		g.judgeWhen(ctx, id, actionWhenMatcher(lookup, id, typ), rows, hidden, byType[typ], keep)
	}
	out := rows[:0]
	for i, r := range rows {
		if keep[i] {
			out = append(out, r)
		}
	}
	return out
}

// judgeWhen sets keep[i] for each i in idx whose row passes m. It fails
// closed as [detailActionCheck.whenHolds] does: a nil m, a condition that
// reads a property hidden on the row, or an evaluation error keeps nothing.
func (g batchActionGate) judgeWhen(
	ctx context.Context, id string, m ViewConditionMatcher, rows []llmBatchRow,
	hidden []map[string]struct{}, idx []int, keep []bool,
) {
	if m == nil {
		return
	}
	attrs, ok := m.(entityAttributeReader)
	if !ok {
		slog.Warn("dataentry: detail action when does not report its fields; not offering it", "action", id)
		return
	}
	page := make([]*entityPkg.Entity, 0, len(idx))
	pageIdx := make([]int, 0, len(idx))
	for _, i := range idx {
		if slices.ContainsFunc(attrs.EntityAttributes(), func(name string) bool {
			_, h := hidden[i][name]
			return h
		}) {
			continue
		}
		page = append(page, rows[i].redacted)
		pageIdx = append(pageIdx, i)
	}
	if len(page) == 0 {
		return
	}
	var gq store.GraphQueryer
	if q, ok := g.svc.store.(store.GraphQueryer); ok {
		gq = q
	}
	verdicts, err := m.MatchPage(ctx, page, traversalGateFromContext(ctx).GateTraversal, pageMatch(gq))
	if err != nil {
		slog.Warn("dataentry: evaluating detail action when failed; not offering it", "action", id, "error", err)
		return
	}
	for j, i := range pageIdx {
		if j < len(verdicts) && verdicts[j] {
			keep[i] = true
		}
	}
}
