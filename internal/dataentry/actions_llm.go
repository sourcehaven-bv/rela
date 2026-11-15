package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

// llmChatTimeout caps one AI call of an `llm:` action. The call runs on the
// request path, so it must end even when the provider stalls.
const llmChatTimeout = 2 * time.Minute

// llmCandidatePage is how many candidate rows one condition evaluation
// judges at once.
const llmCandidatePage = 64

// runLLM runs an `llm:` action on the bound entity and returns its proposal.
// It writes nothing: the caller applies what it accepts through the ordinary
// write APIs, where its own ACL, validation and audit apply.
//
// Every read is the caller's: the subject is the redacted bound row, and
// related entities and candidates come through deps.VisibleReader, which
// gates rows and redacts fields. raw is the stored subject, used only to ask
// which outputs the caller may write; it never reaches the model.
func (o actionOps) runLLM(
	ctx context.Context, s *Schema, id string, action dataentryconfig.Action,
	raw, subject *entityPkg.Entity, deps lua.WriteDeps,
) (*script.ActionResponse, *opFailure) {
	if raw == nil || subject == nil {
		return nil, failV1(http.StatusNotFound, "entity_not_found", "Entity not found")
	}
	task, err := llmtask.New(id, action, s.Meta)
	if err != nil {
		return nil, failLLMInternal(err)
	}
	in := llmtask.Input{
		Subject:   subject,
		Writable:  o.llmWritable(ctx, raw, task.Config().Outputs.Set),
		Relatable: o.llmRelatable(ctx, raw, task.Config().Outputs.Relate),
	}
	if len(in.Writable) == 0 && len(in.Relatable) == 0 {
		return nil, failV1(http.StatusForbidden, "nothing_to_propose",
			fmt.Sprintf("action %q has no output you may change on this entity", id))
	}

	in.Related, err = llmRelated(ctx, deps, s, subject, task.Config().Inputs.Related)
	if err != nil {
		return nil, failLLMInternal(err)
	}
	in.Candidates, err = o.llmCandidates(ctx, deps, s, id, subject, task.Config().Outputs.Relate, in.Relatable)
	if err != nil {
		return nil, failLLMInternal(err)
	}

	// Every output the caller may change is a relation with no readable
	// target: the model could only answer "nothing", so it is not asked.
	hasCandidate := slices.ContainsFunc(in.Candidates, func(c llmtask.Candidates) bool { return len(c.Items) > 0 })
	if len(in.Writable) == 0 && !hasCandidate {
		return &script.ActionResponse{Proposal: actionproposal.Empty(subject.Ref().String())}, nil
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
	req, err := task.Request(in)
	if err != nil {
		return nil, failLLMInternal(err)
	}
	callCtx, cancel := context.WithTimeout(ctx, llmChatTimeout)
	defer cancel()
	resp, err := provider.Chat(callCtx, req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, failLLMInternal(ctxErr)
		}
		// The kind and status only: the provider's message may echo the
		// request, which holds entity data.
		kind, status := ai.ErrKind(""), 0
		if aiErr := (*ai.Error)(nil); errors.As(err, &aiErr) {
			kind, status = aiErr.Kind, aiErr.Status
		}
		slog.Warn("llm action: AI call failed", "action", id, "kind", kind, "status", status)
		return nil, failV1(http.StatusBadGateway, "ai_failed", llmCallDetail(err))
	}
	proposal, err := task.Parse(resp, in)
	if err != nil {
		var re *llmtask.ReplyError
		if errors.As(err, &re) {
			// Not the error text: it quotes values the model chose, which
			// may come from entity data.
			slog.Warn("llm action: unusable AI reply", "action", id)
			return nil, failV1(http.StatusBadGateway, "ai_reply_invalid", re.Error())
		}
		return nil, failLLMInternal(err)
	}
	return &script.ActionResponse{Proposal: proposal}, nil
}

// llmCallDetail describes a failed AI call to the caller. It names the kind
// of failure, never the provider's message, which may echo the request.
func llmCallDetail(err error) string {
	var aiErr *ai.Error
	if errors.As(err, &aiErr) {
		switch aiErr.Kind {
		case ai.ErrTimeout:
			return "The AI provider did not answer in time"
		case ai.ErrRateLimited:
			return "The AI provider is rate limiting requests; try again later"
		case ai.ErrAuth:
			return "The AI provider refused the configured credentials"
		case ai.ErrNotConfigured, ai.ErrBadRequest, ai.ErrServerError, ai.ErrNetwork,
			ai.ErrBadResponse, ai.ErrStreamingUnsupported:
		}
	}
	return "The AI request failed"
}

// failLLMInternal is a 500 whose cause is logged, never shown.
func failLLMInternal(err error) *opFailure {
	f := failV1(http.StatusInternalServerError, "action_failed", "Action failed")
	f.cause = err
	return f
}

// llmWritable is the part of outputs.set the caller may write on raw: the
// update verb holds, and each property is visible and writable to them.
func (o actionOps) llmWritable(ctx context.Context, raw *entityPkg.Entity, set []string) []string {
	if len(set) == 0 {
		return nil
	}
	// Only the update verdict: computeActions would also authorize delete
	// and rename, once per row of a batch run.
	if !o.h.affordances.acl().AuthorizeWrite(ctx, translateVerb("update", raw.Type, raw.ID, raw.Face)).Allow {
		return nil
	}
	fv := o.h.affordances.fieldVerdicts(ctx, raw)
	var out []string
	for _, p := range set {
		if w, ok := fv.Writable[p]; ok && !w {
			continue
		}
		if v, ok := fv.Visible[p]; ok && !v {
			continue
		}
		out = append(out, p)
	}
	return out
}

// llmRelatable is the relation types of outputs.relate the caller may
// create from raw. The verdict map is sparse: an absent type is creatable.
func (o actionOps) llmRelatable(
	ctx context.Context, raw *entityPkg.Entity, relate []dataentryconfig.LLMRelateOutput,
) []string {
	if len(relate) == 0 {
		return nil
	}
	rv := o.h.affordances.outgoingRelationVerdicts(ctx, raw)
	var out []string
	for _, r := range relate {
		if v, ok := rv.Types[r.Relation]; ok && !v.Creatable {
			continue
		}
		out = append(out, r.Relation)
	}
	return out
}

// llmRelated reads inputs.related through the caller's reader: the edges
// whose both ends they may read, then the peers, capped per input.
func llmRelated(
	ctx context.Context, deps lua.WriteDeps, s *Schema, subject *entityPkg.Entity,
	inputs []dataentryconfig.LLMRelatedInput,
) ([][]llmtask.Item, error) {
	out := make([][]llmtask.Item, len(inputs))
	for i, r := range inputs {
		dir := store.DirectionOutgoing
		if !r.Outgoing() {
			dir = store.DirectionIncoming
		}
		var ids []string
		for rel, err := range deps.VisibleReader.ListRelations(ctx, store.RelationQuery{
			EntityID: subject.ID, Type: r.Relation, Direction: dir,
		}) {
			if err != nil {
				return nil, fmt.Errorf("llm action: read %s relations: %w", r.Relation, err)
			}
			peer := rel.To
			if dir == store.DirectionIncoming {
				peer = rel.From
			}
			ids = append(ids, peer)
		}
		ids = slices.Compact(slices.Sorted(slices.Values(ids)))
		if len(ids) > r.EffectiveLimit() {
			ids = ids[:r.EffectiveLimit()]
		}
		if len(ids) == 0 {
			continue
		}
		// One gated read per peer, in the reader's world. The edge list is
		// capped at the input's limit above, so this is bounded.
		for _, id := range ids {
			e, err := deps.VisibleReader.GetAddress(ctx, id)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("llm action: read %s peers: %w", r.Relation, err)
			}
			out[i] = append(out[i], llmtask.NewItem(s.Meta, e, r.Fields))
		}
	}
	return out, nil
}

// llmCandidates reads the candidate list of each relatable relate output:
// targets of the relation's `to` types the caller may read, that pass the
// output's `candidates:` condition, up to its limit. A candidate the subject
// already points at is marked, so the proposal can leave it out.
func (o actionOps) llmCandidates(
	ctx context.Context, deps lua.WriteDeps, s *Schema, id string, subject *entityPkg.Entity,
	relate []dataentryconfig.LLMRelateOutput, relatable []string,
) ([]llmtask.Candidates, error) {
	out := make([]llmtask.Candidates, len(relate))
	var lookup ViewConditionLookup
	for i, r := range relate {
		if !slices.Contains(relatable, r.Relation) {
			continue
		}
		if r.Candidates != "" && lookup == nil {
			var err error
			if lookup, err = o.llmConditions(s); err != nil {
				return nil, err
			}
		}
		existing, err := llmExistingTargets(ctx, deps, subject.ID, r.Relation)
		if err != nil {
			return nil, err
		}
		cands := &out[i]
		for _, target := range s.Meta.Relations[r.Relation].To {
			var m ViewConditionMatcher
			if r.Candidates != "" {
				m = viewConditionFor(lookup, viewKindLLMCandidates,
					dataentryconfig.LLMCandidateConditionID(id, i, target))
				if m == nil {
					return nil, fmt.Errorf("llm action %q: relate[%d] has no compiled condition for %q", id, i, target)
				}
			}
			full, err := o.collectCandidates(ctx, s, subject.ID, target, r, m, existing, cands)
			if err != nil {
				return nil, err
			}
			if full {
				break
			}
		}
	}
	return out, nil
}

// collectCandidates appends the readable rows of type target that pass m to
// cands. It reports whether the limit was reached, in which case cands is
// marked truncated.
func (o actionOps) collectCandidates(
	ctx context.Context, s *Schema, subjectID, target string,
	r dataentryconfig.LLMRelateOutput, m ViewConditionMatcher, existing map[string]bool,
	cands *llmtask.Candidates,
) (bool, error) {
	var page []*entityPkg.Entity
	flush := func() (bool, error) {
		keep, err := o.matchCandidates(ctx, m, page)
		page = page[:0]
		if err != nil {
			return false, err
		}
		for _, e := range keep {
			if len(cands.Items) == r.EffectiveLimit() {
				cands.Truncated = true
				return true, nil
			}
			item := llmtask.NewItem(s.Meta, e, r.Fields)
			item.Related = existing[e.ID]
			cands.Items = append(cands.Items, item)
		}
		return false, nil
	}
	// The list path's gated header read: row-gated in the store query, in
	// the request's world, with no bodies. Each page is redacted once
	// before the condition sees it.
	rqr := readGateFromContext(ctx).ReadQuery(ctx, target)
	headers, _, err := scopedHeaders(ctx, Services{Store: o.h.store}, rqr, scopeRequest{Type: target, Faces: rqr.Faces})
	if err != nil {
		return false, fmt.Errorf("llm action: read %s candidates: %w", target, err)
	}
	red := affRedactor{aff: func() affordanceService { return o.h.affordances }}
	for start := 0; start < len(headers); start += llmCandidatePage {
		raw := make([]*entityPkg.Entity, 0, llmCandidatePage)
		for _, h := range headers[start:min(start+llmCandidatePage, len(headers))] {
			if h.ID != subjectID {
				raw = append(raw, headerEntity(h))
			}
		}
		pctx := visibility.PrimeTraversals(ctx, red, raw)
		for _, e := range raw {
			page = append(page, visibility.Redact(pctx, red, e))
		}
		if full, err := flush(); err != nil || full {
			return full, err
		}
	}
	return false, nil
}

// matchCandidates returns the rows of page that pass m. A nil m admits
// every row. A row on which m reads a property hidden from the caller is
// left out: on the redacted row the property binds as unset, and judging
// that would make the list an oracle on the hidden value.
func (o actionOps) matchCandidates(
	ctx context.Context, m ViewConditionMatcher, page []*entityPkg.Entity,
) ([]*entityPkg.Entity, error) {
	if m == nil || len(page) == 0 {
		return page, nil
	}
	attrs, ok := m.(entityAttributeReader)
	if !ok {
		return nil, errors.New("llm action: candidate condition does not report its fields")
	}
	var gq store.GraphQueryer
	if q, ok := o.h.store.(store.GraphQueryer); ok {
		gq = q
	}
	verdicts, err := m.MatchPage(ctx, page, traversalGateFromContext(ctx).GateTraversal, pageMatch(gq))
	if err != nil {
		return nil, fmt.Errorf("llm action: evaluate candidates: %w", err)
	}
	var out []*entityPkg.Entity
	for i, e := range page {
		if i >= len(verdicts) || !verdicts[i] {
			continue
		}
		if slices.ContainsFunc(attrs.EntityAttributes(), e.IsRedacted) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// llmConditions compiles the config snapshot's conditions, which include
// every `candidates:` predicate.
func (o actionOps) llmConditions(s *Schema) (ViewConditionLookup, error) {
	var compile ViewConditionFunc
	if o.h.affordances.actionConditions != nil {
		compile = o.h.affordances.actionConditions()
	}
	if compile == nil {
		return nil, errors.New("llm action: no condition compiler is wired")
	}
	lookup, problems := compile(s.Cfg, s.Meta)
	if len(problems) > 0 {
		return nil, fmt.Errorf("llm action: conditions did not compile: %v", problems)
	}
	return lookup, nil
}

// llmExistingTargets is the set of entities the subject already points at
// through relation, as far as the caller can see.
func llmExistingTargets(ctx context.Context, deps lua.WriteDeps, subjectID, relation string) (map[string]bool, error) {
	out := map[string]bool{}
	for rel, err := range deps.VisibleReader.ListRelations(ctx, store.RelationQuery{
		EntityID: subjectID, Type: relation, Direction: store.DirectionOutgoing,
	}) {
		if err != nil {
			return nil, fmt.Errorf("llm action: read %s relations: %w", relation, err)
		}
		out[rel.To] = true
	}
	return out, nil
}
