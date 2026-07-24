---
id: FEAT-PPH1EU
type: feature
title: 'ACL-bound read visibility seam: internal/visibility Reader + tracer decorator across data-entry and Lua'
summary: A shared internal/visibility package (PolicyReader/AllowAllReader + a tracer.Tracer visibility decorator) that row-gates and field-redacts every read-out above the store, wired into data-entry export and all Lua reads — the read-containment half of safe scheduled ACL-bound LLM jobs.
description: 'Introduce a shared internal/visibility package — a row-gating, field-redacting Reader (PolicyReader/AllowAllReader) plus a tracer.Tracer visibility decorator — so read-side ACL is enforced structurally at the seam instead of by convention at each consumer. Wired into data-entry export (closes the #1188 IB finding) and all Lua reads; base services (store, tracer, search) stay pure and ungated per DEC-ZBI39P. System jobs keep a genuine system:* principal for audit and receive allow-all as an explicitly wired capability. Enables scheduled ACL-bound LLM jobs: the job role''s reader bounds what can enter a prompt.'
status: in-progress
---

## Feature

Make read-side ACL (entity row-gating + field-level `visible:` redaction)
**structural** across data-entry and Lua by introducing a shared
`internal/visibility` package, per DEC-ZBI39P:

- `visibility.Reader` — `Get`/`Filter` over store reads: row-gate first (hidden = indistinguishable 404), then field-redact a **copy** (incl. the display-title fallback). Impls: `PolicyReader` (composes `acl.Request` + `affordances.PolicyResolver` from the ctx principal) and `AllowAllReader` (pass-through capability for system jobs).
- A **tracer decorator** implementing `tracer.Tracer`: hidden = nonexistent (subtree pruned; `FindPath` through hidden intermediate → no-path indistinguishably; hidden orphans dropped). Base tracer stays pure.
- Wiring: request paths (data-entry export, `export_render:` Lua, actions) get the policy wrappers; scheduler jobs keep a genuine `system:*` principal and receive `AllowAllReader` explicitly (capability ≠ identity, ElevatedManager precedent).

## Why

- Closes PR #1188's blocked IB-review finding (export bypassed field redaction) at the seam, not per-call-site.
- Enables **scheduled ACL-bound LLM jobs**: a job role's reader bounds what can enter a prompt — what never enters the reader never reaches the LLM vendor or a written summary. Composes with TKT-Z1OP7R (egress half).

## Out of scope

MCP reads (follow-up; closes RES-H5AB7S's gap with the same seam), CLI render
(operator trust boundary), per-job role authoring UX, write-back/derivation
provenance governance.

Survey: RES-PSZZKU. Decision: DEC-ZBI39P.
