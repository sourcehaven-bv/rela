---
id: TKT-3Q0GP1
type: ticket
title: 'Postgres derived-schema reconciler (seam + unique rule): atomic unique:true, db status drift, dry-run'
kind: enhancement
priority: medium
effort: l
status: in-progress
---

Follow-up to TKT-ANUJDS (`unmatched_principal: provision`, shipped).
Provisioning creates a stub user entity keyed on the declared
`principal_property = sub`, a `unique:true` property. But `unique:true` is
enforced ONLY by a racy check-then-write scan
(`internal/entitymanager/unique.go` `checkUniqueProperties`) on every backend —
NOT a DB constraint. Two processes racing a first-write for the same subject
both pass the scan and both insert → two stubs with the same `sub` →
`ResolvePrincipal` permanently ambiguous → all future grants for that subject
lost. `unique.go`'s own doc comment already names the fix: "the only race-free
enforcement is a store-level unique index (a partial unique index on pgstore)".

## Reconcile algorithm (load-bearing — doc-worthy)

STATELESS desired-vs-actual convergence. There is NO persisted "current state"
record — the Postgres catalog IS the current state, the metamodel IS the desired
state, the `rela_derived_` name prefix IS ownership. Three inputs, no fourth
copy; introspection reads ground truth so a hand-dropped index self-heals and
reconcile is idempotent by construction. (A versioned migration counter is the
WRONG model — derived DDL is an unordered SET that gains/loses members as YAML
edits, not an ordered forward-only sequence; that's why it can't ride the
migration runner.)

Per rule (unique is rule #1; each rule owns its OWN sub-prefix, e.g.
`rela_derived_uniq__`):
```
desired = { hash(T,P) : (T,P) in metamodel where P.unique && !P.list, charset-valid }
actual  = { indexname : pg_indexes WHERE schemaname=current_schema()
                        AND indexname LIKE 'rela_derived_uniq__%' }   # prefix-scan, this rule only
DROP   every name in actual \ desired    # scanned by prefix, matched no current hash → operator removed the flag
CREATE every hash in desired \ actual    # declared but absent (may fail on dupes → unenforced+warn)
no-op  the intersection                  # steady state → zero DDL
```

- **hash(T,P)** = `rela_derived_uniq__` + hex(truncated SHA-256 over canonical `T\x00P`), ≤63 bytes. DETERMINISTIC across processes/versions (no per-run salt) so the drop side is safe; `\x00` separator so `(ab,c)` and `(a,bc)` can't collide. One-way, so the reverse lookup (ConstraintName→property on a 23505) is done by RECOMPUTING all current-metamodel hashes and matching — no persisted registry (RR-B5Y6DZ).
- **Per-rule sub-prefix ownership**: the unique reconciler only ever drops `rela_derived_uniq__*`; a future enum rule owns `rela_derived_enum__*`. Rules can't clobber each other; the general reconciler runs each rule's diff independently.
- Wrapped in `pg_advisory_xact_lock(reconcileKey, hashtext(current_schema()))` (leader does DDL, others fast-path); `schemaname=current_schema()` filter (no cross-schema drop); each CREATE carries the empty-exempt predicate.
- The returned `[]Outcome` is held IN MEMORY on the store handle only so `rela db status` can report the last reconcile — a cache of a computation, not a source of truth (status/--dry-run recompute `plan()` fresh). The only thing that outlives a process is the indexes themselves, which IS the enforcement.

## Design (settled — see PLAN-SG4MA1; design-reviewed)

The metamodel already contains a pipeline of future DDL (unique→index,
enum→CHECK, regex→CHECK, relation-cap→unique, from/to→CHECK), all sharing one
failure mode: an ADD that fails on pre-existing violating rows. So build the
general seam once, seeded with ONE rule. Mirrors `startVersionSweepIfSupported`
(build-tagged, metamodel-aware, optional-capability post-open hook; type-assert
the store; `//go:build !postgres` no-op stub).

1. **`DerivedSchemaReconciler`** optional store capability; the convergence above; universal outcome `enforced|created|dropped|unenforced{reason,blockingCount}`. General, not provision-scoped.
2. **unique rule DDL**: partial expression index, EMPTY-EXEMPT to match the scan (`WHERE type='t' AND properties->>'p' <> '' AND properties->>'p' IS NOT NULL` — RR-AROZJY).
3. **Constraint-name discriminator** (VERIFIED by spike, RR-5LZWX8): on 23505 branch on `pgErr.ConstraintName` — PK/`entities_id_lower_key` → `store.ErrConflict` (409, unchanged); `rela_derived_uniq__*` → `store.ErrUniqueProperty{Property}` on BOTH insert and update paths (RR-CWI8HG). entitymanager maps that → the SAME `ValidationErrorUnique` (422, property named, value withheld) the scan produces. Scan stays primary (friendly + fs/mem); index is the atomic race backstop. NEVER surface pgErr.Detail (it echoes the colliding value — enumeration oracle).
4. **DDL injection guard** (RR-GVXUIQ): charset validator for property/type names at metamodel load + reconciler refuses out-of-charset names + literal escaping.

**Degrade (all rules):** a failed ADD on pre-existing violations → object stays
`unenforced`, logged loud (blocking COUNT; values only under operator
`--show-values`), CONTINUE — never a startup outage.

**Surfaced, not silent:** `rela db status` (operator-shell-only, no ACL — STATED
trust boundary) reports enforced/unenforced/stale, exits 0 always; `rela db
reconcile` re-converges; `rela db reconcile --dry-run` prints the plan and exits
NON-ZERO on drift (the CI/pre-flight gate). One `plan()` backs store-open,
status, and dry-run — dry-run prediction == store-open behaviour.

## Scope

IN: `DerivedSchemaReconciler` capability + stateless `plan()` (prefix-scan +
set-diff on hashed names, per-rule sub-prefix, schema-filtered, advisory-locked)
+ universal outcome; the ONE `unique` rule (empty-exempt);
`store.ErrUniqueProperty` + `ConstraintName` branch on insert AND update;
metamodel-load charset validator; entitymanager mapping to
`ValidationErrorUnique`; appbuild wiring (`assemble` + `!postgres` stub) +
postgres MCP wiring; `pgstore.Status`→struct; `rela db status` (exit 0) + `rela
db reconcile` [+ `--dry-run` exit-non-zero, `--show-values`]; degrade-not-crash;
pgstore-gated tests.

OUT (future ~1-rule tickets on the SAME seam): enum→CHECK; regex→CHECK;
relation-cap→unique/exclusion; from/to→CHECK; required→JSONB CHECK. Also OUT:
fs/mem atomic uniqueness (stays check-then-write; documented); IdP-webhook vs
lazy-provision reconciliation (separate follow-up); rewriting the scan.

## Acceptance criteria

1. On postgres, two concurrent inserts of the same `unique:true` value → exactly one row; the loser fails atomically at COMMIT (not via the pre-scan).
2. Failure → `store.ErrUniqueProperty` → entitymanager `ValidationErrorUnique` (422, property named, value withheld); ID-collision path STILL returns `ErrEntityAlreadyExists` (409); raw pgErr.Detail never surfaced.
3. Provision case: the loser's `maybeProvision` catches the conflict and re-resolves to the winner's stub.
4. Reconcile: adding `unique:true` creates the index at next store-open; removing it DROPS the owned index; idempotent; steady-state = zero DDL; per-rule sub-prefix (no cross-rule clobber); schema-filtered (no cross-schema drop); advisory-locked.
5. Pre-existing duplicates do NOT crash startup — object stays unenforced, warning names the blocking COUNT (values only under --show-values).
6. `rela db status` reports enforced/unenforced/stale, exits 0; `reconcile --dry-run` prints the same plan and exits NON-ZERO on drift; `plan()` shared by store-open/status/dry-run.
7. Scan and index AGREE on empty, absent, list, and non-string values (dual-enforcement contract).
8. fs/mem behaviour unchanged (check-then-write scan; documented caveat).
9. Out-of-charset property/type name is refused at metamodel load (and by the reconciler).
10. Update-path 23505 (automation-set unique prop) maps correctly; create-then-failed-automation-update outcome documented.
