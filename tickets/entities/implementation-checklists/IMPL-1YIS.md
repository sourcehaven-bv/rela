---
id: IMPL-1YIS
type: implementation-checklist
title: 'Implementation: Create-form field affordances: default _fields verdicts for an unsaved entity'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written — Go: `TestValidateCreate_*` (entitymanager), `TestHandleV1DryRunCreate_*` (dataentry). TS: `stagedEntity.test.ts`, `dryRunCreate.test.ts`.
- [x] Integration tests — dry-run tests drive the real `handleV1EntityCollection` HTTP handler through the real `entitymanager.ValidateCreate` (httptest), asserting `_fields` shape + persist-nothing + no-audit. Browser-verified end-to-end (see evidence).
- [x] Happy path — allowed create succeeds (existing create tests still green); dry-run returns 200 with verdicts.
- [x] Edge cases — hidden field omitted; read-only disabled; enum option filtered; value-dependent re-derivation (debounced); stale-drop (AbortController); fail-open on dry-run error; commit sends only visible+writable keys; server applies hidden/read-only defaults post-gate (RR-SIA6).
- [x] Error handling — dry-run hard errors → 422; SPA fails open on dry-run failure; commit re-authorizes (advisory verdicts, RR-Y85M).

## Test Quality

- [x] Fixture builders — Go uses `newVerdicts()` builder + `newManager`; TS mocks `api.post`.
- [x] No hardcoded values where object in scope — wire `rule_id`/codes asserted literally (wire-contract exception per CLAUDE.md).
- [x] Only test-relevant values specified.
- [x] ~~Interpolated values from objects~~ (N/A: no interpolation in these assertions).
- [x] ~~Property comparisons use original object~~ (N/A: assert HTTP shape, not preserved props).

## Manual Verification

- [x] Feature manually tested end-to-end (browser).
- [x] Each acceptance criterion verified.
- [x] Edge cases manually verified.

**Verification Evidence:** Ran `rela-server` against a demo project
(`.ignored/tkt3i5u-demo`) with an `everyone` role: closed-world `fields:`
(status read-only), `visible:` (secret hidden), `options:` (priority `high`
denied).
- Dry-run API (`POST /api/v1/tickets?dry_run=true`) returned `_fields: {status:{writable:false}, priority:{options:{high:false}}}`, `status:open` default applied, `secret` absent, and entity count stayed at 1 (no persist).
- Create form (`/form/create_ticket`) in browser: `status` select `disabled:true`; `priority` option `high` `disabled:true` ("high (not allowed)"); `secret` field not rendered. Screenshot captured.
- `go test ./internal/dataentry ./internal/entitymanager/...` PASS; `npm run test:run` 847 pass; `npm run typecheck`/`lint`/`build` clean.

## Quality

- [x] Follows project patterns — dry-run mirrors `/_templates` endpoint precedent + reuses `serializeEntityForWire`; SPA reuses edit-mode's affordance filter + autosave's AbortController pattern.
- [x] DRY — `buildCandidateEntity` extracted so `createCore` (persists) and `ValidateCreate` (dry) share the exact defaults+validation path (zero drift, RR-Y85M); no premature abstraction.
- [x] No security issues — dry-run is fail-closed for relation/entity-scoped predicates, advisory-only, commit re-authorizes; sentinel `++new++` is form-only and never sent (asserted).
- [x] No silent failures — dry-run errors surface; SPA fail-open is deliberate (commit gate is boundary, RR-HUQ3).
- [x] No debug code left behind.
