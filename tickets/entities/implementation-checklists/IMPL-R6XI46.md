---
id: IMPL-R6XI46
type: implementation-checklist
title: 'Implementation: Gate the analyze reader: run validation through the requester visible reader (arc step 1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written — `TestACLAnalyze_GatedReadClosesMessageLeak` (the sentinel leak), plus existing title/templated/row tests now pass via gating not filtering.
- [x] Integration tests — all drive `handleV1Analyze` end-to-end through the real gate under a Declarative `visible:` policy.
- [x] Happy path implemented — all three read seams (readDeps.VisibleReader, validator candidate load, analyzeService) unified onto one gated `lateGatedReader`; output filter removed.
- [x] Edge cases handled — construction-time acl capture (→ lateGatedReader, late-bound); nop-ACL (Unrestricted, no gating); relation COUNTS stay raw (structural, cannot leak).
- [x] Error handling in place — gate-construction fault REFUSES (DenyReader, fail-closed); reads inherit PolicyReader/ScriptReader fail-closed semantics.

## Test Quality

- [x] Using fixture builders — `newAppFromParts` / `seedEntity` / `mustNewACL` / `gateCtxFor`.
- [x] ~~No hardcoded values in assertions~~ (N/A: sentinel leak-checks need a literal probe).
- [x] Only specifying values that matter — a hidden property + a distinctive value + (for the message test) an invalid enum value.
- [x] ~~Interpolated values~~ (N/A: sentinel).
- [x] ~~Property comparisons use original object~~ (N/A: sentinel).

## Manual Verification

- [x] Feature manually tested — neutralized the gating (test reader → raw) and confirmed `TestACLAnalyze_GatedReadClosesMessageLeak` FAILS, leaking `Invalid value "SECRET-STATUS"` in the message; restored → passes.
- [x] Each acceptance criterion verified — AC1 (message + title value absent), AC2 (hidden entity → no issue), AC3 (full-visibility identical: full suite passes).
- [x] Edge cases verified — the two bugs found and fixed (see Evidence).

**Verification Evidence:** The message-leak test bites: without gating the
response contains `"message":"Invalid value \"SECRET-STATUS\" (allowed: [open
closed])"`; with gating the value is absent (entity redacted before
ValidateEntity). Two real bugs surfaced and were fixed during implementation:
(1) `gatedScriptReader` captured `aclImpl` at construction → stale when
`app.acl` is reassigned → added `lateGatedReader` resolving `a.acl` live per
call; (2) test `rebindApp` wired analyze to the raw store → didn't exercise
gating → rewired to `lateGatedReader`. Full `internal/dataentry` +
`internal/validator` + `internal/validation` green under `-race`;
`golangci-lint` 0 issues; `just plimsoll` + `just arch-lint` clean.

## Quality

- [x] Code follows project patterns — reuses the `ScriptReader`/`PolicyReader`/`ctxRowGate` seam (DEC-ZBI39P); consumer-side interfaces (`analyzeReader`, `EntityLister`) per the CLAUDE.md call-site-interface rule.
- [x] Checked for DRY — extracted `gatedScriptReader` (shared by scriptReader + wiring); `lateGatedReader` is the single late-binding wrapper.
- [x] No security issues introduced — removes a disclosure; every path fails closed. Order-safe: reads gated in the same change that drops the filter.
- [x] No silent failures — gate faults REFUSE and log.
- [x] No debug code left behind — the neutralize probe was reverted.
