---
id: RR-0VHKMW
type: review-response
title: 'No bound on the roles array: unbounded per-request work and audit growth from a token'
finding: 'Nothing in the plan caps len(roles) or per-element length. A signature-verified token means the IdP said it, not that it is small — a compromised or buggy IdP, or pathological group membership, yields a very large roles array. Per request that is N map lookups in computeGlobals (cheap), N clean() calls plus a proportionally huge JSONL audit line (not cheap), and N entries in the attribution slice flowing into Decision.Attributions and denial diagnostics. The existing discipline is right there: sanitizeUser''s 256-rune cap (dataentry/router.go:465, principalUserMaxLen). The bound belongs in the jwtauth projection, not in dataentry, so every future consumer inherits it rather than the next entry point missing it.'
severity: significant
resolution: 'Accepted. Bounds added in the jwtauth projection (NOT in dataentry), so every future consumer inherits them rather than the next entry point silently missing the cap — the reviewer''s placement argument is right. Concretely: cap the roles array at 32 elements and each element at the existing principalUserMaxLen (256 runes), mirroring sanitizeUser''s discipline at dataentry/router.go:465; drop the excess and log once at slog.Warn so truncation is visible rather than silent. Rationale recorded on the ticket: signature verification proves the IdP asserted the claim, not that it is bounded — a buggy or compromised IdP, or pathological group membership, otherwise turns one request into N map lookups, N clean() calls and a proportionally huge JSONL audit line.'
status: addressed
---

## Finding

The plan places no cap on `len(roles)` or on per-element length. "Verified"
means the IdP asserted it, not that it is bounded — a compromised or buggy IdP,
or a user with pathological group membership, produces a very large array.

Per-request consequences: N map lookups in `computeGlobals` (cheap), N `clean()`
calls plus a proportionally huge JSONL audit line (not cheap), and N entries in
the attribution slice, which flows into `Decision.Attributions` and denial
diagnostics.

## Resolution

Mirror the existing discipline — `sanitizeUser`'s 256-rune cap
(`dataentry/router.go:465`, `principalUserMaxLen`). Cap element count (32 is
generous for a role set) and per-element length, drop the excess, log once at
`slog.Warn`.

**Put the bound in the `jwtauth` projection, not in `dataentry`**, so every
future consumer inherits it instead of the next entry point silently missing it.
