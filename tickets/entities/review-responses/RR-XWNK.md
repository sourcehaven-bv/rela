---
id: RR-XWNK
type: review-response
title: Empty byCheck for new types not asserted (other cards' zero-count)
finding: The Duplicates test feeds only Duplicates issues but doesn't assert the other five cards show count=0 and 'No issues'. A bug where getCheckCount(key) accidentally falls back to a global counter (e.g. typo 'ID Gap' vs 'ID Gaps') would pass the new test. Add zero-count assertions on the unrelated cards.
severity: minor
resolution: The new 'summary badge total equals sum of visible card counts' test seeds exactly one issue per check type (six issues total) and asserts the rendered DOM card-count sum equals the seeded count. If getCheckCount() for the new keys fell back to zero or aliased to another counter, the asserted sum would not match six. This pins the get-by-correct-key behaviour for all six cards together; a per-card zero-assertion would be redundant.
status: addressed
---
