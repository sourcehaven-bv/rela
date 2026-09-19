---
id: RR-GPC7NH
type: review-response
title: Scope is larger than the ticket's goal requires; the state-store seam and the file-format change are separable
finding: 'The ticket''s stated goal is data-only migrations plus a committed applied-list, but the plan bundles four independent changes: the per-backend state service, removing hashes from files, removing embedded projections, and timestamp naming. Only removing the from/to hashes is strictly required for data-only migrations. Given RR-SJISFW shows removing projections is actively harmful and the effort is already l with a large test rewrite, recommend splitting into two tickets so the data-only capability can land and be validated before the storage relocation.'
severity: minor
reason: Jeroen chose to keep it as one ticket (2026-09-19). The split's main motivation was isolating the riskiest item, and that item (removing embedded projections) has been struck from scope via RR-SJISFW, so the remaining work is materially smaller and less entangled than when this was raised. Test churn is now confined to hash and naming assertions rather than a rewrite of the file format.
status: wont-fix
---

## Finding

The ticket's goal is stated plainly: enable data-only migrations, track
migrations by name. The plan bundles four changes that are independent:

1. **Per-backend migration-state service** (replacing `state.KV`).
2. **Remove `from:`/`to:` hashes** from migration files.
3. **Remove embedded projections** from migration files.
4. **Timestamp naming** replacing `%04d`.

Only **(2)** is strictly required for data-only migrations — the blocker is
`file.go:75-77` refusing `from == to`, and that refusal exists only because the
hashes exist.

Given RR-SJISFW establishes that (3) is actively harmful, and that the effort is
already `l` with the largest cost being a test rewrite touching nearly every
file in the package, the bundle is worth reconsidering.

## Suggested split

**Ticket A — data-only migrations (effort m).** Drop `from:`/`to:` from files;
keep the embedded projections (per RR-SJISFW); make the applied-list the sole
position mechanism in its EXISTING `state.KV` home; timestamp naming. Delivers
the user-visible capability and the BUG-TY2XQC numbering fix. Test churn is
confined to file-format and resolve tests.

**Ticket B — migration-state storage relocation (effort m).** The per-backend
service: `filemigstate` committed to `migrations/`, `pgmigstate`,
`sqlitemigstate`, conformance harness, the server-read-only split, the
`state.KV` transition and its rollback story (RR-2HRIHC). Delivers the
fresh-clone fix. Purely internal relocation with no file-format coupling.

B depends on A only loosely; either order works.

## Why this is worth doing

- The two halves fail differently. A is a format change with a mechanical test
rewrite. B touches four backends, the appbuild recipes, a new pg DDL migration,
and an upgrade/downgrade path. Bundled, a problem in B blocks the capability in
A.
- The riskiest single item (`validateDeltasResolved`, RR-SJISFW) sits in A, and
it is much easier to reason about when the storage location is not also moving
underneath it.
- `l` tickets with a large test rewrite are where review fatigue sets in, and the
security-relevant parts (RR-J064T6's name validation, RR-2HRIHC's rollback) are
in B where they can get proper attention.

## Severity

Minor — this is a workflow judgement, not a correctness defect. The plan as
written is implementable. Recording it because the split materially lowers risk
and the decision is cheap to make now and expensive to make halfway through.

## Evidence

- `internal/datamigration/file.go:75-77` — the actual blocker, tied to hashes only
- TKT-XCJ0Y2 problem statement — goal is data-only migrations + name-keyed list
- RR-SJISFW — item (3) should not happen at all
- RR-2HRIHC — the upgrade/downgrade story lives entirely in item (1)
