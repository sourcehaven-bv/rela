---
id: DOCS-RLHSGF
type: docs-checklist
title: 'Docs: JWT identity must fail closed, never downgrade to --principal-header'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code docs

- [x] `requireVerifiedJWT` godoc explains why a gate and not a chained resolver
- [x] `NewRouter` carries a CRIT-1-voice comment on the ordering hazard, naming
why BOTH alternatives (outermost / innermost) are wrong
- [x] `ErrKeysUnavailable` godoc states it deliberately does not wrap `ErrInvalid`
- [x] `classify` godoc documents the bias toward `ErrInvalid` and why
- [x] `isAPIPath` godoc carries the RR-P2M7 bare-`/api` rationale
- [x] `SetJWTGate` godoc documents the interface-typed-nil caveat
- [x] `logSampler.sample` godoc documents the trailing-residual limitation and
points to `noteRecovery`
- [x] Stale `New` godoc clause about "the chain's fall-through identity" corrected
- [x] `JWTPrincipalResolver` marked Deprecated with the reason

## Project docs

- [x] `docs/server-security.md`: replaced the "Chain order" paragraph — it
documented exactly the behavior being removed
- [x] Added "Mutually exclusive" (startup error + the attacker-triggerable
downgrade rationale)
- [x] Added "Fail-closed behavior" (401 semantics, ungated SPA shell + why)
- [x] Added "Availability trade-off" — separates "transient blip is invisible"
from "rotation during outage is an outage", discloses the 5s stall, and gives
rotation-staging guidance
- [x] Corrected the `--jwt-jwks-url` bullet ("rotation needs no restart" was
materially incomplete)
- [x] Added a forward pointer from the `--principal-header` trust-boundary block
- [x] Corrected the rate-sampling overclaim found in review ([[RR-LHFXGZ]]) and
disclosed that classification is heuristic

## External docs

- [x] ~~API reference~~ (N/A: no new or changed API endpoints; the gate changes
the auth requirement for existing `/api/` routes, which is a deployment concern
covered in server-security.md)
- [x] ~~Changelog~~ (N/A: not maintained in this repo)
