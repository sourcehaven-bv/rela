---
id: TKT-3FL2S6
type: ticket
title: 'Gate the analyze reader: run validation through the requester visible reader (arc step 1)'
kind: refactor
priority: high
effort: m
status: done
description: Step 1 of the gated-analyze arc ([[TKT-270KRY]]) — the security fix. Make `_analyze` read through the requesting principal's visible reader so a validation rule cannot read data the requester cannot see; the message-leak the sentinel caught becomes impossible by construction. Removes the `visibleAnalysisIssues` output filter (and the earlier title/message redaction patches) — gating replaces post-hoc filtering.
---

## Plan

- **Validator reader:** swap the data-entry validator's `ReadDeps.VisibleReader`
from `visibility.Unrestricted(st)` (app.go:~565) to a ctx-gating reader — the
`ctxRowGate` / `visibility.PolicyReader` seam already used by the view fix, so
the gate resolves per-request from ctx. Validator is still built once; only its
reader becomes ctx-aware. `CheckRuleFull(ctx, …)` already threads ctx.
- **analyze.go non-Lua checks:** route the ~6 raw `svc.store` reads (Orphans,
Duplicates, ID-Gaps, Cardinality, Properties) through a ctx-gated reader so they
see only the requester's slice.
- **Remove** `visibleAnalysisIssues` and the provenance-era title-fallback /
message-suppression in api_v1.go — redundant once reads are gated.

## Acceptance

- A `visible:`-hidden property value never appears in any `_analyze` response
(the sentinel's analyze case passes).
- A hidden ENTITY produces no analyze issues for a principal who cannot read it
(unchanged from today's row-gate).
- Full-visibility principals see identical analyze output to today (gating is a
no-op for them).

## Known / accepted (fixed in step 2)

Whole-graph built-in checks (Cardinality, Orphans) may FALSE-POSITIVE for a
partial-visibility principal (a required neighbor is merely hidden). Accepted
and documented; the `roles:` rule annotation (step 2) is the applicability
guard. Nop-ACL deployments are unaffected (everyone sees everything → no gating
effect).
