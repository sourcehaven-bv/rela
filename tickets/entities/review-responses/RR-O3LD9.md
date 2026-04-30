---
id: RR-O3LD9
type: review-response
title: Backend tests miss whitespace-q, special chars, type pinning, and pagination
finding: 'Per PLAN-XYB07 Edge Cases: whitespace-only q (q=%20%20%20), quoted phrases, q+pagination interaction, and the silent-drop of prop:status=open behavior all lack regression tests. fakeSearcher also ignores q.Types so the type-pinning behavior is not verified.'
severity: significant
resolution: 'Extended TestV1ListEntitiesSearchQuery from 4 to 10 sub-tests covering: whitespace-only q (no-op), prop-only q without free-text words (ignored), searcher type-pinning (q.Types=ticket even with stray type:feature), searcher error → 500, q + pagination (total reflects post-q count, page slice from filtered set), quoted phrase forwarding. fakeSearcher now records gotTypes for type-pinning assertions.'
status: addressed
---
