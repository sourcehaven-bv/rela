---
id: RR-3ZJM3P
type: review-response
title: 'Load-bearing but uncommented: subjects buffering and non-nil empty slice'
finding: (a) checkCardinality buffers every (subject, count) before emitting — deliberate, it preserves the historical min-then-max grouped ordering — but nothing said so; a future optimizer could collapse it to a single pass and silently reorder output. (b) CheckCardinality's `make([]CardinalityViolation, 0)` lost its nolint comment; the non-nil-empty initialization is what keeps JSON Details serializing as [] rather than null, and nothing recorded that.
severity: minor
resolution: 'Both now carry explanatory comments in internal/analysis/analysis.go: the buffering comment names the single-pass reordering hazard the pinning tests guard, and the make() carries ''// Non-nil even when empty: JSON callers serialize Details as [], not null.'' The interleaving regression itself is now also pinned by TestCheckCardinality_MinMaxGroupedAcrossTypes.'
status: addressed
---
