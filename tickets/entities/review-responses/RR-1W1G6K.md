---
id: RR-1W1G6K
type: review-response
title: GraphQuery pushdown would drop field-level redaction
finding: 'PLAN-VZXHRJ proposes replacing PolicyReader.Filter with a store.GraphQuery pushdown on the list path. Filter (policyreader.go:66) does TWO jobs: the batched row gate AND redacted(ctx,c) on every survivor. store.GraphQuery expresses only the row gate — it has no concept of field-level `visible:` policy (verified: no redaction/visible/Inaccessible references in internal/store/graphquery.go). Implementing the plan as written returns every property a visible: policy hides, in full, to any script calling list_entities. That is the exact CISO finding (#1188) that started the FEAT-PPH1EU arc, reintroduced by its own follow-up.'
severity: critical
resolution: 'PLAN-VZXHRJ Approach rewritten: the pushdown replaces the ROW GATE ONLY; redaction still runs per returned row on every branch. Added AC9 asserting a visible:-hidden property is absent from every returned row, explicitly testing PROPERTIES rather than row identity — the pre-existing row-gate tests would have passed against an un-redacting implementation, which is why this was catchable only by reading Filter''s body. Benefit claim in the plan corrected: the pushdown removes the PermitsReadMany probe, not the per-row redaction copy.'
status: addressed
---

Raised by `/design-review` against PLAN-VZXHRJ, before implementation.

The pushdown is still the right idea — it removes the `PermitsReadMany`
amplification and the short-page problem. But it replaces **only the row gate**.
Redaction is a separate concern that must still run per returned row.

Consequence for the plan's claimed benefit: we avoid the ACL probe, NOT the
per-row redaction copy. The memory story is therefore weaker than the plan
implies, on top of the already-noted absence of store-side paging.
