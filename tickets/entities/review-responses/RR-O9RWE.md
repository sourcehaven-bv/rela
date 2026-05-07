---
id: RR-O9RWE
type: review-response
title: Migration creates permanent hybrid state on conflict; not idempotent
finding: 'Plan: conflicting detail_view values across lists for same type -> warn, leave untouched. But Detect() will keep returning true (lists still have detail_view), so server keeps refusing to start. Forever. Detect() must return true only when Apply() will actually change something; conflict-warning path needs a different surface (e.g. rela validate). Plan must add an idempotency test: run migration twice, second pass is no-op.'
severity: critical
resolution: 'Detect() is now idempotent: returns true only when Apply() will produce a non-empty change. Conflicting-value groups are skipped entirely (not flagged at migrate time); after migration of all migrate-able groups, only conflicts remain and Detect() returns false. Conflict surfacing moved to a future rela validate enhancement (out of scope). Idempotency test added: run migration twice, second pass = no-op.'
status: addressed
---
