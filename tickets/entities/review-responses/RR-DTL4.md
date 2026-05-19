---
id: RR-DTL4
type: review-response
title: Promote sanitization helpers to internal/principal
finding: sanitizeUser (dataentry) and clean/truncateRunes/isControlRune (audit) implement the same policy. The audit twin still has the same allocation pattern and dead r >= 0 check this PR fixed in the dataentry copy. The right home is internal/principal (or a new internal/textsan) as a SanitizeField(s, limit) string exported helper.
severity: significant
reason: 'Cross-package extraction touches the audit''s sanitization contract, its sanitization tests, and the JSONL wire-format guarantees. That''s a separate refactor with its own review surface; shouldn''t ride along on the per-request-Principal ticket. Cranky flagged this as "Leverage" rather than blocking. Follow-up: file a separate ticket once TKT-WEBI ships.'
status: deferred
---
