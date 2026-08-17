---
id: PLAN-SG4MA1
type: planning-checklist
title: 'Planning: Postgres derived-schema reconciler (seam + unique rule)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Design Review — OUTCOME (2026-08-16)

Ran `/design-review`: my own read of the create path + an adversarial
cranky-reviewer grounded in the codebase. **2 critical + 7 significant + 3
minor** findings. The core bet (partial unique index as atomic backstop;
check-then-write scan stays as friendly primary) is SOUND and the create-path
transaction ordering is clean (index rejection returns before audit/event —
verified in core.go:111-126, audit fires after createCore returns). But 9
substantive findings reshape the plan; captured as review-response entities
RR-5LZWX8, RR-GVXUIQ, RR-AROZJY, RR-CWI8HG, RR-QY5S4C, RR-2HMGZJ, RR-FTQE3U,
RR-B5Y6DZ, RR-3NB0P9 (all `open`).

**Two criticals gate implementation — DE-RISK BEFORE ANY DDL:**
- **RR-5LZWX8** (ConstraintName): the discriminator (route entities_id_lower_key vs rela_derived_uniq__* by pgErr.ConstraintName) is UNVERIFIED for expression indexes. entities_id_lower_key is itself an expression index, so it's expression-vs-expression. If pgx v5 leaves ConstraintName empty for expression-index 23505, the whole error-mapping collapses. → **SPIKE: prove pgx populates ConstraintName for an expression unique-index violation before committing.** Fallback if empty: this design can't distinguish the two sources by name and must be rethought (e.g. a sentinel column, or SAVEPOINT+re-query, or accept ID-only ErrConflict).
- **RR-GVXUIQ** (DDL injection): property/type names are NOT charset-validated at load (verified: validatePropertyDefs, loader.go:655, checks reserved+type only, no [A-Za-z0-9_]+ regex). Names are interpolated into DDL string literals (can't bind-param CREATE INDEX). → add a strict charset validator at metamodel load (independently good hygiene) + reconciler refuses any name outside charset + proper literal escaping.

**Plan revisions from the significant findings (all folded into Approach
below):**
- **RR-AROZJY** empty-string: index predicate MUST be `WHERE type='t' AND properties->>'p' <> '' AND properties->>'p' IS NOT NULL` to match the scan's empty-exempt semantics; add scan-vs-index agreement test.
- **RR-CWI8HG** update path: add isUniqueViolation+ConstraintName to the UpdateEntity path too (automation-set unique props are a second 23505 source); document the create-then-failed-automation-update outcome.
- **RR-QY5S4C** + **RR-FTQE3U** locking: wrap reconcile plan→apply in `pg_advisory_xact_lock(reconcileKey, hashtext(current_schema()))` (new key). Leader-does-DDL; others fast-path. Steady-state boots do ZERO DDL (skip when already converged) — this also resolves the DROP-INDEX ACCESS-EXCLUSIVE lock hazard (non-concurrent DDL is fine inside the lock when it's a rare no-op; CONCURRENTLY-vs-tx tension avoided).
- **RR-2HMGZJ** cross-schema: introspection MUST filter `schemaname = current_schema()` (pg_indexes is DB-global; shared-DB multi-schema is supported).
- **RR-B5Y6DZ** registry: recompute candidate hashes from the CURRENT metamodel and match the incoming ConstraintName (no persisted registry); unmappable rela_derived_uniq__* → safe generic 409/422 (no property), never 500/mis-attribution.
- **RR-3NB0P9** leak: `db status`/`reconcile` are operator-shell-only (no ACL, like db migrate) — STATE that boundary; samples NEVER on an API/health surface; default to a COUNT of blocking groups, values only behind `--show-values`.

**Minors folded in (no separate entities):**
- (#10) split exit codes: `db status` always exit 0 (health/liveness); `reconcile --dry-run` exits NON-ZERO on drift (the CI gate). Don't bundle both onto status.
- (#11) `pgstore.Status`/`StatusDSN` return a STRUCT, not a growing tuple, so the derived-schema drift extension doesn't ripple again.
- (#13) add an explicit AC/test pinning the dual-enforcement contract: scan and index agree on empty, absent, list, and non-string values (per-type scoping already agrees).

- [x] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan — **BLOCKED on the two critical spikes (RR-5LZWX8, RR-GVXUIQ) + user go-ahead**

**Design Review Findings:** RR-5LZWX8, RR-GVXUIQ (critical); RR-AROZJY,
RR-CWI8HG, RR-QY5S4C, RR-2HMGZJ, RR-FTQE3U, RR-B5Y6DZ, RR-3NB0P9 (significant);
#10/#11/#13 (minor, folded).

---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Reframe:** This is the FIRST place schema.yaml drives real Postgres DDL. Today
Postgres backs rows only; every domain constraint (unique, enum, cardinality,
from/to) is app-enforced. The metamodel already contains a pipeline of future
DDL, all sharing one failure mode (an ADD that fails on pre-existing violating
rows). Decision (user-confirmed): build the **general seam** — a
`DerivedSchemaReconciler` doing desired-state convergence — seeded with ONE rule
(`unique`), plus the status/reconcile/dry-run surface that settles
degrade/drift/drop ONCE for every future rule.

**Scope:**

IN: `DerivedSchemaReconciler` optional store capability; desired-state `plan()`
(create-missing + drop-extra over `rela_derived_*`, schema-filtered,
advisory-locked); universal per-object outcome `enforced | unenforced{reason,
blockingCount}`; the single `unique` rule (empty-exempt partial expression
index); `store.ErrUniqueProperty` sentinel + `ConstraintName` branch on BOTH
insert and update paths (pending RR-5LZWX8 spike); metamodel-load charset
validator for property/type names; entitymanager mapping to
`ValidationErrorUnique`; appbuild wiring (`assemble` + `//go:build !postgres`
stub) + postgres MCP wiring; `pgstore.Status` → struct carrying derived drift;
`rela db status` (exit 0) + `rela db reconcile` [+ `--dry-run`
exit-non-zero-on-drift, `--show-values`]; degrade-not-crash safety;
pgstore-gated tests.

OUT (future ~1-rule tickets on the SAME seam): enum `values` → CHECK;
custom-type regex `validations` → CHECK; relation `max_outgoing/max_incoming` →
per-side unique/exclusion; relation `from/to` → type CHECK; `required` → JSONB
CHECK. Also OUT: fs/mem atomic uniqueness (stays check-then-write; documented
caveat); IdP-webhook vs lazy-provision reconciliation (separate follow-up);
rewriting `checkUniqueProperties`.

**Acceptance Criteria:** see ticket TKT-3Q0GP1 (revised set incl. the review
findings). Each maps to a Test Scenario below.

## Research

- [x] Three agent passes: pgstore facts, go-architect design, metamodel→DDL survey; plus adversarial design review. (See prior-art list retained below.)

**Existing Solutions / prior art (file:line):**
- Precedent: `startVersionSweepIfSupported` (appbuild.go:947; postgres impl versionsweep_postgres.go:40; stub versionsweep_nosweep.go:13; store entry pgstore/sweep.go:103) — build-tagged optional-capability post-open hook, type-asserts *pgstore.Store, metamodel-driven, degrades gracefully.
- Properties = ONE JSONB blob (0001_init.sql) → expression indexes/CHECKs over properties->>'p'.
- entities_id_lower_key (0007) — the one existing expression unique index; 23505→ErrConflict at migrate.go:162 / entity.go:253; ConstraintName UNUSED today.
- ErrConflict→ErrEntityAlreadyExists keyed on ID: core.go:121-124, apply.go:179-180, rename.go:75-76.
- Scan + its own doc naming the fix: unique.go:38-107 (empty-exempt at :69; per-type scan EntityQuery{Type:e.Type} at :78).
- create path has NO wrapping tx; audit after createCore returns (core.go:82-129) → clean index-rejection ordering.
- Advisory-lock convention: migrate.go:66 (migrateAdvisoryLockKey), sweep/purge (sweepAdvisoryLockKey), writes (writeAdvisoryLockKey).
- Multi-schema DB-global hazard: feed.go:28-53. Unqualified SQL / search_path isolation.
- charset gap: validatePropertyDefs loader.go:655 (no name regex).
- `rela db status`: db_postgres.go (exits 1 when behind); pgstore.Status returns (current,target int) migrate.go:120 — no drift slot.

## Approach

- [x] Technical approach chosen, revised for review findings

**Technical Approach (revised):**

0. **PRE-WORK / spike (gate):** prove `pgconn.PgError.ConstraintName` is populated for an expression unique-index violation under pgx v5 (RR-5LZWX8). If not, redesign the discriminator before any other code.
1. **metamodel load:** add a strict `[A-Za-z0-9_]+` (or documented safe charset) validator for property AND type names in `validatePropertyDefs` (loader.go) — independently good; the reconciler additionally REFUSES (unenforce+warn) any name outside charset (defense in depth) (RR-GVXUIQ).
2. **`internal/store`:** `DerivedObjectSpec{Kind, Type, Property}`; `DerivedSchemaReconciler` optional capability (`Reconcile(ctx, meta, ReconcileOpts{DryRun,ShowValues}) ([]DerivedObjectOutcome, error)`); `DerivedObjectOutcome{Spec, State: enforced|created|dropped|unenforced, Reason, BlockingCount, SampleValues(opt)}`; `ErrUniqueProperty{Property}` sentinel.
3. **pgstore `derivedschema.go`** (new): under `pg_advisory_xact_lock(reconcileKey, hashtext(current_schema()))` (RR-QY5S4C): desired = rules(metamodel) with names charset-checked; actual = introspect `pg_indexes WHERE schemaname = current_schema() AND indexname LIKE 'rela_derived_%'` (RR-2HMGZJ); diff → create-missing / drop-extra; **skip when converged (steady-state = zero DDL)** (RR-FTQE3U). unique rule DDL: `CREATE UNIQUE INDEX IF NOT EXISTS rela_derived_uniq__<hash> ON entities (type, (properties->>'<prop>')) WHERE type='<type>' AND properties->>'<prop>' <> '' AND properties->>'<prop>' IS NOT NULL` (RR-AROZJY), name hashed ≤63B, literals escaped (RR-GVXUIQ). A failed create → sample blocking COUNT (values only if ShowValues) → unenforced, warn, continue (never fatal).
4. **pgstore `entity.go`:** on 23505 in BOTH CreateEntity and UpdateEntity (RR-CWI8HG), inspect ConstraintName: PK/entities_id_lower_key → ErrConflict; rela_derived_uniq__* → ErrUniqueProperty{prop}. Recover prop by recomputing candidate hashes from the current metamodel (no persisted registry); unmappable → safe generic ErrConflict, no property (RR-B5Y6DZ).
5. **entitymanager:** map ErrUniqueProperty → ValidationError{ValidationErrorUnique, Property} at create AND update choke points; document create-then-failed-automation-update (row persists, audited as created) (RR-CWI8HG); update unique.go doc comment.
6. **appbuild:** `reconcileDerivedSchemaIfSupported(st, base.meta)` in assemble + `//go:build !postgres` no-op stub + mcp_wiring_postgres.go. Store-open runs a live reconcile; retains outcomes on the store for status.
7. **cli db_postgres.go:** `pgstore.Status` → returns a STRUCT incl. derived drift (RR-#11); `runDBStatus` prints enforced/unenforced/stale, **always exit 0**; `runDBReconcile` (+ `--dry-run` **exit non-zero on drift** = CI gate (RR-#10), + `--show-values`). Operator-shell-only, no ACL, STATED trust boundary (RR-3NB0P9).

**One planner, three surfaces** unchanged: store-open reconcile, status, dry-run
share `plan()`.

**Files:** metamodel/loader.go (+validation.go charset);
internal/store/store.go; pgstore/{derivedschema.go(new),entity.go,migrate.go};
entitymanager/{core.go,apply.go,manager.go,unique.go}; appbuild/appbuild.go +
derivedschema_{postgres,nosweep}.go(new); cli/db_postgres.go +
mcp_wiring_postgres.go; storetest/; pgstore tests
(RELA_TEST_DATABASE_URL-gated).

**Alternatives rejected:** static migration (metamodel-blind); Tx-serialized
scan (cross-process lock cliff, violates no-slow-IO-in-Tx); unique-only
capability (re-opens seam at next rule); provision-only (name is config
regardless); status-non-zero-on-unenforced (bricks CI on dirty data — user
rejected; strict exit lives on --dry-run instead).

## Security Considerations

- [x] DDL injection is the one real surface (RR-GVXUIQ): charset validator at load + reconciler refusal + literal escaping; never raw-concat.
- [x] Enumeration-oracle: blocking values are entity content (secret); operator-shell-only surface, count-by-default, --show-values opt-in, never on API/health (RR-3NB0P9). 422 still withholds the colliding value (preserve scan behaviour).
- [x] Uniqueness is itself a security property → degrade must be LOUD (status + dry-run), never silent.

## Test Plan

- [x] AC-mapped scenarios + the review-driven ones:
- Spike test: expression-index 23505 → ConstraintName non-empty (RR-5LZWX8).
- Concurrent insert → single row, loser ErrUniqueProperty (AC1).
- Mapping → ValidationErrorUnique; ID-collision STILL ErrEntityAlreadyExists (AC2 + regression).
- Empty/absent/list/non-string: scan and index AGREE (RR-AROZJY, #13).
- Update-path 23505 from automation-set unique prop mapped correctly (RR-CWI8HG).
- Provision e2e conflict→re-resolve (AC3).
- Reconcile create/drop/idempotent/converged-noop (AC4); cross-schema isolation (RR-2HMGZJ); advisory-lock serialization (RR-QY5S4C).
- Pre-existing dupes → store opens, unenforced, warned, no crash (AC5).
- db status exit 0 + dry-run exit-non-zero-on-drift + samples gated behind --show-values (AC6, RR-3NB0P9, #10).
- Malicious property name refused (RR-GVXUIQ). fs/mem unchanged (AC7).

## Risk Assessment

- [x] [CRITICAL-GATE] ConstraintName unverified (RR-5LZWX8) → spike first.
- [x] [CRITICAL] DDL injection (RR-GVXUIQ) → charset validator + escaping.
- [x] [HIGH] pre-existing violations → outage → degrade+dry-run pre-flight.
- [x] [HIGH] silent unenforced = false security property → loud status/dry-run.
- [x] [MED] 2-source 23505 misclassification, cross-schema drop, boot-DDL lock, registry miss, update-path gap, leak → all addressed above.

**Effort:** l (unchanged; the criticals add a spike + a small
metamodel-validator, not new subsystems).

## Documentation Planning
- [x] GUIDE-acl-security.md (provision caveat closed on pg), postgres-backend guide (derived-DDL namespace, status/reconcile/dry-run, degrade, operator dry-run-before-deploy workflow, trust boundary), cli-reference.md, metamodel.md (unique DB-enforced on pg + the new name charset rule), unique.go doc comment. Regenerate docs/ via `just docs`.
