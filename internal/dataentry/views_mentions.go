package dataentry

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// mentionStepPages bounds the backlink pages one `mentioned_by` step reads
// while its `where:` keeps rejecting sources. At the step limit of 200 that
// is 2,000 readable sources examined per view, on top of the resolver's own
// per-page scan budget.
const mentionStepPages = 10

// collectBacklinks fills a `mentioned_by` step's collection with the entry's
// backlinks (TKT-HF0D5W).
//
// Sources come from [visibility.Resolver.MentionedBy] on the view reader, so
// each one is row-gated and served at the face the world admits. Their ids
// are then reloaded as raw headers through [viewsHandler.loadViewEntities],
// like every other traversal: `where:` filters raw values, and the view
// redacts each collection exactly once, on the way out.
//
// `where:` runs page by page, so the step limit counts matching sources.
// The collection is marked incomplete when more matching sources may exist.
//
// A gate or store fault, in the index read or in the reload, fails the view as
// a [gateFaultError]. Returning an empty or short collection would
// read as "nothing mentions this", while the view hides the built-in panel
// that could have shown otherwise.
func (h *viewsHandler) collectBacklinks(ctx context.Context, rule ViewTraverse, result *viewResult, w viewWorld) error {
	limit := dataentryconfig.MentionedByStepLimit
	collected := []*entity.Entity{}
	result.Collections[rule.CollectAs] = collected
	res := h.viewReader.Resolver()
	if res == nil { // coverage-ignore: every production view reader has a resolver
		return nil
	}
	vw := worldHandle(w).visibility()
	s := h.schema()
	after := ""
	for range mentionStepPages {
		page, err := res.MentionedBy(ctx, vw, result.Entry.ID, after, limit)
		if errors.Is(err, visibility.ErrMentionsUnsupported) { // coverage-ignore: every store backend keeps the index
			return nil
		}
		if err != nil {
			return &gateFaultError{err: err}
		}
		ids := make([]string, 0, len(page.Items))
		for _, item := range page.Items {
			ids = append(ids, item.ID)
		}
		found, err := h.loadViewEntities(ctx, ids, w)
		if err != nil {
			return &gateFaultError{err: err}
		}
		if rule.Where != "" {
			if found, err = filterEntities(s, found, rule.Where); err != nil {
				return err // refused at load; reaching here is a config bug
			}
		}
		for _, e := range found {
			if len(collected) == limit {
				result.markIncomplete(rule.CollectAs, collected)
				return nil
			}
			collected = append(collected, e)
		}
		result.Collections[rule.CollectAs] = collected
		if page.Truncated {
			result.markIncomplete(rule.CollectAs, collected)
			return nil
		}
		if page.Next == "" {
			return nil
		}
		after = page.Next
	}
	result.markIncomplete(rule.CollectAs, collected)
	return nil
}

// markIncomplete stores the collection and flags it incomplete.
func (r *viewResult) markIncomplete(name string, collected []*entity.Entity) {
	r.Collections[name] = collected
	if r.Incomplete == nil {
		r.Incomplete = map[string]bool{}
	}
	r.Incomplete[name] = true
}

// propagateIncomplete marks every collection traversed from an incomplete
// one as incomplete too: a step walking from a capped backlinks set misses
// whatever the uncollected sources lead to. `from: "*"` reads every
// collection, so it inherits any incompleteness.
func (r *viewResult) propagateIncomplete(rules []ViewTraverse) {
	for changed := len(r.Incomplete) > 0; changed; {
		changed = false
		for _, rule := range rules {
			if rule.MentionedBy || r.Incomplete[rule.CollectAs] {
				continue
			}
			if r.Incomplete[rule.From] || (rule.From == "*" && len(r.Incomplete) > 0) {
				r.Incomplete[rule.CollectAs] = true
				changed = true
			}
		}
	}
}
