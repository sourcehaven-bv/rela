---
id: RR-2TPCTX
type: review-response
title: 'EffectiveBound exported an implementation detail for one warning message'
finding: 'A one-line exported wrapper over an unexported function, added so the migration generator could reuse the nil-normalization for a WARNING comment in a generated draft. Widens a package''s public API to serve one caller, and the (p *int, isMax bool) signature uses a boolean that changes what the function means.'
severity: minor
reason: 'Deferred to follow-up: it is a quality or ergonomics issue, not a correctness one, and it does not touch the data-loss paths this ticket exists to close. Filed rather than fixed so the reversal change stays reviewable.'
status: deferred
---
