---
id: PLAN-6B7S1Z
type: planning-checklist
title: 'Planning: Author-aware version capture: last_edited_by column + flush-on-author-change (precise per-version attribution)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** (narrowed by design review, 2026-07-24 — flush-on-author-change split
to TKT-0IGI4V)

IN scope:

- Real principal attribution on sweep-captured **entity** create/update versions (the screenshot case: `unknown · version-sweep`).
- Same for **relation** create/update versions (same sweep, `sweep.go:310`/`410`).
- Migration adding nullable `last_edited_by_user`/`last_edited_by_tool` to `entities` + `relations`; pgstore stamps them on create/update writes.
- Store-owned `store.Attribution{User, Tool}` + ctx carrier, populated only at the entitymanager boundary from a REAL (non-zero/non-unknown) principal.
- Fallback: NULL columns (legacy rows, principal-less writes) → sweep keeps the `version-sweep` system principal — never guess, never stamp literal "unknown".
- Postgres backend only.

NOT in scope:

- **Flush-on-author-change** → follow-up TKT-0IGI4V (per RR-K781MZ; carries pinned semantics from RR-VG4BPJ/RR-4OJAC1/RR-MMDQ3N/RR-MZ4PPG/RR-MORL7M/RR-12HJ4K). Consequence accepted for v1: two different authors editing within one debounce window still merge into ONE swept version, attributed to the LAST author (a strict improvement over `version-sweep`).
- Backfilling existing version rows (audit-log correlation remains the recovery path).
- fsstore/memstore behavior changes; read-API/UI exposure of last-editor.

**Acceptance Criteria:**

1. **AC1 — swept update attribution:** authenticated user X edits an entity via data-entry; after the sweep captures, the version row has `principal_user = X`, `principal_tool = data-entry` (not `version-sweep`).
2. **AC2 — swept create attribution:** same for a freshly created entity's `create` version.
3. **AC3 — same-author debounce preserved:** user X edits 3× within the window → still ONE swept version (attributed to X).
4. **AC4 — fallback:** a write with no/zero/unknown principal leaves columns NULL; sweep attributes `version-sweep`; nothing errors. Literal "unknown" is never stamped.
5. **AC5 — relations:** a relation prop/body edit by user X yields a swept relation version attributed to X.
6. **AC6 — rename re-key neutrality:** RenameEntity's bulk relation re-key leaves `last_edited_by_*` untouched.
7. **AC7 — no regressions:** storetest conformance, sweep dedup, purge guardrails, change-feed watermark all unaffected.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: design carried in ticket body since 2026-07-08; open decisions settled with user + /design-review)
- [x] ~~Searched for existing libraries~~ (N/A: internal storage/attribution mechanics)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (RES-4ILUJZ covered versioning storage shape)

**Existing Solutions:**

- Sweep system-principal stamping to replace: `internal/store/pgstore/sweep.go:310` (entities), `sweep.go:410` (relations).
- Sync capture path with real principals (rename/delete): `entitymanager.VersionRecorder` (`internal/entitymanager/manager.go:172-181`), adapters in `internal/appbuild/appbuild.go:755-800`.
- Principal plumbing: `internal/principal` (ctx-based; data-entry stamps per-request). NOTE: `principal.From` returns `{unknown, unknown}` for unstamped ctx — boundary must IsZero-check (RR-U964M0).
- External prior art: collaborative-editor revision histories segment by author (relevant to the split-out flush ticket).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Migration `0006_last_edited_by.sql`: nullable `last_edited_by_user text` /
`last_edited_by_tool text` on `entities` AND `relations` (mirroring
`principal_user`/`principal_tool` on version tables; NULL = unknown/legacy; no
DEFAULT).

1. **Store-owned attribution concept (user decision, 2026-07-24):** `store.Attribution{User, Tool}` + `store.WithAttribution(ctx, a)` / `store.AttributionFrom(ctx)` in `internal/store`. The entitymanager write boundary translates the ctx Principal → `store.WithAttribution` **only when the principal is real** (`IsZero()`/unknown → no attribution → NULL columns; RR-U964M0). pgstore never imports `internal/principal`; the CLAUDE.md invariant is preserved and its wording extended to name the Attribution carrier as the second sanctioned boundary-populated input (RR-2VWA0Q). fs/mem stores ignore the ctx value.
2. **Stamp on write (pgstore):** `CreateEntity`/`UpdateEntity`/`CreateRelation`/`UpdateRelation` read `store.AttributionFrom(ctx)`, set columns (NULL when absent). RenameEntity's bulk re-key statements untouched — they must not clobber the columns (RR-U1RGSE).
3. **Sweep reads the columns:** candidate queries select `last_edited_by_*`; `captureOne`/`captureRelation` stamp `PrincipalUser`/`PrincipalTool` from them, falling back to `{tool: "version-sweep"}` when NULL.

**Placement decision (RESOLVED with user):** pgstore-contained via the
store-owned Attribution ctx concept. Domain-field alternative
(`entity.Entity.LastEditedBy`) rejected: would drag `canonical.HashEntity`,
storetest conformance, fsstore frontmatter, and all backends into a pg-only
feature.

**Files to modify:**

- `internal/store/store.go` — `Attribution` type + ctx helpers
- `internal/entitymanager` write paths — Principal → `WithAttribution` (IsZero-guarded)
- `internal/store/pgstore/migrations/0006_last_edited_by.sql` (new)
- `internal/store/pgstore/entity.go`, `relation.go` — stamp columns
- `internal/store/pgstore/sweep.go` — select + stamp, NULL fallback
- `CLAUDE.md` + docs postgres-backend guide — attribution route, fallback, rename-neutrality
- Tests: DB-gated pgstore tests (attribution stamped / NULL both directions; swept-version principal; fallback), migration status test if count-asserted

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Principal from ctx (data-entry middleware ← oauth2proxy headers), translated to `store.Attribution` at the entitymanager boundary. Trust model identical to the audit log's ("a forged principal under an untrusted proxy would forge last_edited_by too — document, don't re-solve"). Opaque text, parameterized SQL only.
- No new user-controlled surface: columns written from boundary-populated ctx, read only by the sweep.

**Security-Sensitive Operations:**

- Version rows gain real usernames where they had a system tag — the point of the ticket; matches sync rename/delete versions. History reads already ACL-gated. Purge semantics untouched.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** (DB-gated via `RELA_TEST_DATABASE_URL`, `just
test-postgres`)

- AC1/AC2: write with `WithAttribution` on ctx → force sweep tick → assert version principal_user/tool.
- AC3: three writes same attribution → one sweep → exactly one new version.
- AC4: write with no attribution / boundary given zero-or-unknown principal → columns NULL → swept version carries `version-sweep`; assert no "unknown" literals.
- AC5: relation update with attribution → attributed swept relation version.
- AC6: rename entity with incident relations → relations' `last_edited_by_*` unchanged.
- AC7: existing conformance + sweep + purge + ordering suites pass unchanged.
- Contract pinning (RR-2VWA0Q): store-level test asserting both directions of the Attribution ctx contract.

**Edge Cases:**

- Partial attribution (user set, tool empty or vice versa): store as given — columns are independent; sweep falls back only when BOTH NULL? No — fallback when user AND tool both NULL; partial rows stamp what exists. Keep simple: treat attribution as a unit — boundary only sets it when principal is real, so partials shouldn't occur; sweep falls back unless at least one column is non-NULL.
- Legacy rows (pre-migration): NULL → fallback, self-heals on next edit.
- Unicode/long usernames (email addresses): text columns, parameterized — fine.
- Concurrent writes to same entity: last write's attribution wins on the row — matches "last editor" semantics by definition.
- Migration on live deployment: nullable ALTER, no backfill, instant.

**Negative Tests:**

- Zero/unknown principal at boundary → no WithAttribution call (unit-testable in entitymanager).
- Sweep with mixed NULL/non-NULL candidates in one tick → each version attributed independently.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Silent degradation** (RR-2VWA0Q): a write path missing `WithAttribution` yields NULL → fallback attribution, no error. Mitigation: entitymanager is the single choke point for human writes; contract tests pin behavior; enumerate non-entitymanager store-write callers during implementation.
- **"unknown" literal leakage** (RR-U964M0): guarded by IsZero check + explicit negative test.
- **Sweep query cost:** two extra selected columns on existing candidate scans — negligible.
- **v1 author-merge limitation:** two authors in one window merge to last-author attribution until TKT-0IGI4V lands — documented, accepted.

Effort: **s/m** (reduced by the split — one migration, store ctx helper,
pgstore-local stamping, DB-gated tests).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] CLAUDE.md — content-versioning bullet: Attribution ctx carrier as second boundary-populated input; NULL fallback; rename-neutrality; face to TKT-0IGI4V for the flush
- [x] docs postgres-backend guide — attribution semantics + fallback
- [ ] ~~docs/metamodel.md~~ (N/A)
- [ ] ~~docs/cli-reference.md~~ (N/A)
- [ ] ~~docs/data-entry.md~~ (N/A: UI simply starts showing real names)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** (go-architect agent, 2026-07-24)

- RR-U964M0 (critical, addressed): IsZero-guard the boundary translation — never stamp literal "unknown".
- RR-2VWA0Q (critical, addressed): pin the Attribution ctx contract with tests + CLAUDE.md wording update.
- RR-K781MZ (significant, addressed): flush split to TKT-0IGI4V.
- RR-VG4BPJ / RR-4OJAC1 / RR-MMDQ3N / RR-MZ4PPG (significant, deferred → TKT-0IGI4V): flush atomicity / dedup backing / op choice / purge-tombstone reasoning.
- RR-MORL7M / RR-12HJ4K (minor, deferred → TKT-0IGI4V): no rename markers / pure decision seam.
- RR-U1RGSE (minor, addressed): rename bulk re-key leaves columns untouched; documented.
