package store

import (
	"context"
	"fmt"
	"slices"
)

// RollupCond is the store form of a list condition's `entity.p == true` or
// `entity.p == false` on an exists roll-up p (design gap 3.5, step 2): a
// walk seeded with the candidate row alone, gated for the reader as the
// roll-up column is. Whether the walk reached a counted row is Holds.
//
// A walk over its row cap leaves the roll-up absent, and an absent leaf
// fails closed: a row with an absent leaf anywhere in its Where tree is no
// match, whatever the leaf's polarity and whether it sits under a NOT.
//
// Query is an along or siblings [RollupQuery] with [RollupCount] and no
// Sources or visits: every candidate is its own source.
type RollupCond struct {
	Query RollupQuery
	Holds bool
}

// RollupLeaf is a [CondRollup] leaf.
func RollupLeaf(q RollupQuery, holds bool) Cond {
	return Cond{Op: CondRollup, Rollup: &RollupCond{Query: q, Holds: holds}}
}

// Judge reports whether the leaf holds for a source whose walk gave res;
// a source without a result reached nothing. known=false: the source is
// over its row cap, so the leaf is absent.
func (c RollupCond) Judge(res RollupResult) (holds, known bool) {
	if res.OverCap {
		return false, false
	}
	return (res.Count > 0) == c.Holds, true
}

// validateRollupCond refuses a roll-up leaf whose query is not one walk
// per candidate.
func validateRollupCond(c *RollupCond) error {
	switch {
	case c == nil:
		return fmt.Errorf("%w: a roll-up leaf needs its query", ErrInvalidQuery)
	case c.Query.Along == nil && c.Query.Siblings == nil:
		return fmt.Errorf("%w: a roll-up leaf walks along or siblings", ErrInvalidQuery)
	case len(c.Query.Sources) > 0:
		return fmt.Errorf("%w: a roll-up leaf is seeded by the candidate, not by sources", ErrInvalidQuery)
	case c.Query.Func != RollupCount:
		return fmt.Errorf("%w: a roll-up leaf counts", ErrInvalidQuery)
	case c.Query.Along != nil && c.Query.Along.Visits:
		return fmt.Errorf("%w: a roll-up leaf returns no visits", ErrInvalidQuery)
	}
	return ValidateRollupQuery(c.Query)
}

// RollupLeafChunk is the most candidates one batched answer of a roll-up
// leaf asks [Rollup] about.
const RollupLeafChunk = 1000

// RollupLeafAnswers answers every roll-up leaf of where for ids, one
// [Rollup] call per leaf per [RollupLeafChunk] ids: per leaf, whether it
// holds for each id whose answer is known. An id over its row cap is
// absent from its leaf's map; the caller drops its row (see [RollupCond]).
// Each id is capped alone, so the other ids of its chunk keep their
// answers. A backend that cannot render the leaf uses it (graphquerynaive).
func RollupLeafAnswers(
	ctx context.Context, st RollupReader, where Cond, ids []string,
) (map[*RollupCond]map[string]bool, error) {
	out := map[*RollupCond]map[string]bool{}
	var err error
	where.Walk(func(c Cond) bool {
		if c.Op != CondRollup || c.Rollup == nil {
			return true
		}
		if _, done := out[c.Rollup]; done {
			return true
		}
		var held map[string]bool
		held, err = rollupLeafHolds(ctx, st, *c.Rollup, ids)
		out[c.Rollup] = held
		return err == nil
	})
	return out, err
}

func rollupLeafHolds(ctx context.Context, st RollupReader, c RollupCond, ids []string) (map[string]bool, error) {
	held := map[string]bool{}
	for chunk := range slices.Chunk(ids, RollupLeafChunk) {
		q := c.Query
		q.Sources = chunk
		res, err := rollupOn(ctx, st, q)
		if err != nil {
			return nil, err
		}
		for _, id := range chunk {
			if holds, known := c.Judge(res[id]); known {
				held[id] = holds
			}
		}
	}
	return held, nil
}

// RollupLeavesKnown reports whether every roll-up leaf of where has a
// known answer for id in answers, as [RollupLeafAnswers] gives them.
func RollupLeavesKnown(answers map[*RollupCond]map[string]bool, where Cond, id string) bool {
	return where.Walk(func(c Cond) bool {
		if c.Op != CondRollup {
			return true
		}
		_, known := answers[c.Rollup][id]
		return known
	})
}

// HasRollupLeaf reports whether c holds a [CondRollup] leaf.
func HasRollupLeaf(c *Cond) bool {
	if c == nil {
		return false
	}
	return !c.Walk(func(n Cond) bool { return n.Op != CondRollup })
}

// rollupOn is [Rollup] over a [RollupReader].
func rollupOn(ctx context.Context, st RollupReader, q RollupQuery) (map[string]RollupResult, error) {
	if r, ok := st.(Rollupper); ok {
		return r.Rollup(ctx, q)
	}
	return RollupGeneric(ctx, st, q)
}
