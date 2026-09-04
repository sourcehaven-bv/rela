---
id: TKT-YVREQN
type: ticket
title: 'Runtime under the load line: extract elevation/output/schema-sort clusters (45 → ~37)'
kind: refactor
priority: medium
effort: s
status: done
---

Sub-ticket of [[TKT-N0IKN9]], third step of the `lua.Runtime` arc after
[[TKT-4WBLG6]] (105 → 60) and [[TKT-DOPCTI]] (60 → 45). Stacked on TKT-DOPCTI's
branch. This step takes Runtime under the 40-method load line.

## What

**Pure structural extraction, no behavior change, no exported-API change.**

- **Elevation** (−2): `newElevatedHandle` becomes a free function taking a
`func() context.Context` (it already passes callerCtx that way to
`registerElevatedWrites`/`registerElevatedReads`, which are ALREADY free
functions); `luaBypassACL` moves to a narrow binding struct over the three
elevated deps it reads. The elevation SEMANTICS (recorder, elevated
reader/manager wiring) move verbatim — this is receiver plumbing only.
- **Output/filesystem** (−3): `luaOutput`, `luaWriteFile`, `luaPrint` →
`outputBindings` over `{stdout, outputDir, projectRoot, isAction, isDocument}`.
The mode flags are read-only after construction — verify, and capture by value
only if truly immutable; otherwise closure.
- **Schema/sort** (−3): `luaGetEntityTypes`, `luaGetRelationTypes` → a small
binding struct over `deps.Meta`; `luaSortEntities` touches nothing → free
function.

Registration seams stay on Runtime. Ratchet `//plimsoll:max-methods` 45 → ~37
(actual count). Runtime is then UNDER the 40 load line; the directive stays
(pinning the count) with a history line noting the arc goal is met — deleting it
outright is a later decision once the remaining clusters (reads/writes/
lifecycle/registration, all deliberately kept) are confirmed stable.

## Done when

plimsoll with lowered directive; full suite + `-race ./internal/lua/` green;
arch-lint/comment-lint/lint clean. The ACL-elevation tests (bypass_acl/elevated)
pass unchanged.
