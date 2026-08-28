---
id: RR-FDV0GO
type: review-response
title: Elevated-read audit row claims reads that never touched the store
finding: 'readGuard marked the binding as used before each method''s argument validation ran. The empty-string checks (empty id for get_entity, empty type for list_entities) raise BEFORE the reader is called, so a closure doing only `pcall(function() admin.get_entity("") end)` produced an acl-bypass-read audit row claiming a disclosure that never happened. Verified empirically: audit calls = [[get_entity list_entities]] with zero store contact. This contradicted TestElevatedRead_DeniedReadIsNotAudited, whose own comment states the principle — the test only covered the nil-reader denial, so the argument-validation denial slipped through the same hole the test was written to close.'
severity: significant
resolution: Removed reads.mark from readGuard; each of the three methods now calls reads.mark immediately before its er.* call, so the mark happens where disclosure actually occurs. readGuard's godoc now explains why it deliberately does NOT mark. Added TestElevatedRead_DeniedByArgValidationIsNotAudited; mutation-verified by restoring the mark to readGuard, which fails the new test.
status: addressed
---

Raised by the cranky-code-reviewer against commit `31813351`.

The `readGuard` comment justified early marking with "a read that panics or
raises mid-iteration still happened" — true for `list_entities`, but argument
validation happens *before* any store contact, so the justification did not
cover that case. Marking at the store call preserves the mid-iteration property
(the mark still precedes the `er.*` call, so a partial read that then fails is
still recorded) while dropping the false-positive.
