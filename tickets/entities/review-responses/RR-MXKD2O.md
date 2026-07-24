---
id: RR-MXKD2O
type: review-response
title: 'acl.Request churn: wrappers only amortized/snapshot-consistent when wiring attaches a Request — requirement undocumented'
finding: 'DeclarativeGate.request() reuses a ctx-attached acl.Request else opens a fresh one per call — so one Filter/FilterRelations/filterTree over N types opens N Requests, and PolicyRedactor→FieldVerdicts opens yet another. When wiring forgets acl.WithRequest: (a) the Globals member-of walk re-runs per type per collaborator (the exact cost RR-JJYW amortized); (b) the row-gate and field-redactor can evaluate against different Requests, so one read operation is not a single ACL snapshot. Godoc documents the mechanism but not the wiring REQUIREMENT.'
severity: significant
resolution: 'Added DeclarativeGate.Bind(ctx) — opens ONE acl.Request for the ctx principal and attaches it (reuses an existing bound scope; errors on unstamped principal). Godoc now states the wiring REQUIREMENT loudly: one Bind per logical operation, else per-collaborator Request churn + non-atomic snapshots. Suite case BindScopesOperation pins reuse-same-ctx, redaction-through-bound-ctx, and unstamped-deny.'
status: addressed
---
