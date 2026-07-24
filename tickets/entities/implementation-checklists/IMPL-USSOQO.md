---
id: IMPL-USSOQO
type: implementation-checklist
title: 'Implementation: Author-aware version capture: last_edited_by column + flush-on-author-change (precise per-version attribution)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Changes (branch `feat/tkt-zirmgm-version-attribution`):

- `internal/store/attribution.go` (new): `store.Attribution{User, Tool}` + `WithAttribution`/`AttributionFrom` ctx carrier; godoc pins the boundary-populated contract and the NULL-when-absent rule.
- `internal/entitymanager/attribution.go` (new): `withStoreAttribution` boundary helper, IsZero/unknown-guarded (RR-U964M0); called first in all six public write entry points (`ApplyEntity`, `ApplyRelation`, `CreateEntity`, `UpdateEntity`, `CreateRelation`, `UpdateRelation`), so nested automation/cascade/renumber writes inherit it.
- `internal/store/pgstore/migrations/0006_last_edited_by.sql` (new): nullable, DEFAULT-less `last_edited_by_user/_tool` on `entities` + `relations`; header documents NULL=unknown and rename-re-key neutrality.
- `internal/store/pgstore/entity.go` / `relation.go`: `attributionValues(ctx)` helper (empty component → NULL); Create/Update for entities and relations stamp the columns. Rename bulk re-key untouched (RR-U1RGSE) — verified it only SETs id/from_id/to_id/updated_at/seq/search_text.
- `internal/store/pgstore/sweep.go`: candidate queries select the columns; `sweepAttribution` stamps real editor onto swept versions, `{tool: "version-sweep"}` fallback when both NULL.
- `internal/store/pgstore/status_test.go`: migration target 5 → 6.
- Docs: CLAUDE.md content-versioning bullet (two sanctioned attribution routes, fallback, TKT-0IGI4V pointer); `docs/postgres-backend.md` attribution paragraph.

## Test Quality

- [x] Using fixture builders or factories for test data (`mkEntity`, `attributedCtx`, `stubProvider` reused)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

End-to-end verification ran against a real local PostgreSQL (per-test scoped
schemas, `postgres://…/rela_test`), race detector on:

- AC1/AC2 (`TestSweepAttributesRealEditor`): entity + relation written with attribution, backdated, real sweep goroutine captured versions with `principal_user=alice@example.com`, `principal_tool=data-entry`. PASS.
- AC3 (`TestSweepAttributesLastEditorOfBurst`): alice-then-bob burst → ONE version attributed to bob (documented v1 semantics; author segmentation = TKT-0IGI4V). PASS.
- AC4 (`TestSweepAttributesRealEditor` WHO-2 + `TestAttributionColumnsStamped`): unattributed writes leave NULL columns; swept version falls back to `version-sweep` with empty user; no "unknown" literals. Boundary guard unit-tested in `TestWithStoreAttribution` (unstamped ctx, explicit {unknown,unknown}, real principal, unknown-user/real-tool). PASS.
- AC5 (`TestSweepAttributesRealEditor`): relation version attributed. PASS.
- AC6: rename re-key SQL inspected — does not touch the columns (`entity.go:393` id re-key; relation re-key SETs endpoints only).
- AC7: full DB-gated pgstore suite (conformance, sweep, purge, ordering, tombstone, tx stress) green: `ok … 24.6s`. Default-build `internal/store/... internal/entitymanager/...` green with `-race`.

Quality gates: `just lint` 0 issues (after fixing a containedctx finding in my
own test), `just arch-lint` OK, `just plimsoll` OK, `just coverage-check` PASS
(76.0% total), `just build-check-tags` all three tag combos compile.

Note: the reporting deployment's existing v1/v2 rows stay `version-sweep` by
design (no backfill); they self-heal on the next attributed edit.

## Quality

- [x] Code follows project patterns (mirrors principal.With/From ctx idiom; sweep helper mirrors existing capture structure)
- [x] Checked for DRY opportunities — `attributionValues` + `sweepAttribution` extracted once each; no premature abstraction
- [x] No security issues introduced (opaque text, parameterized SQL; trust model unchanged, documented)
- [x] No silent failures (absent attribution is a DOCUMENTED expected state, not a swallowed error; sweep capture errors still logged per existing contract)
- [x] No debug code left behind
