---
id: REV-SLSR
type: review-checklist
title: 'Review: Create-form field affordances: default _fields verdicts for an unsaved entity'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full `just ci` exit 0 (run before commit/push).
- [x] Lint clean — Go (golangci-lint + arch-lint) clean; frontend lint passes (warnings only, all pre-existing patterns).
- [x] Coverage maintained — Go: new tests cover the new code; new files (`stagedEntity.ts`, dryRunCreate test) not yet in baseline (added post-merge). Pre-existing `schema.ts` drift is environmental, identical to develop, not blocking.

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer) — done; 6 findings recorded.
- [x] All critical review-responses addressed — N/A (none raised).
- [x] All significant review-responses addressed — 3 of 3: RR-2U2D (userTouched), RR-8I07 (skip ID-gen scan), RR-00VT (allFields in initializeDefaults).
- [x] Self-reviewed diff — staged-entity model is contained to DynamicForm; dry-run handler reuses serializeEntityForWire and the new buildCandidateEntity seam in entitymanager.

**Review Responses:** RR-2U2D (addressed), RR-8I07 (addressed), RR-00VT
(addressed), RR-9JOH (addressed, minor), RR-2PZB (addressed, minor), RR-GOR8
(addressed, nit). All 6 closed.

## Acceptance Verification

- [x] AC1 (read-only disabled): browser-verified — `status` select disabled in the create form.
- [x] AC2 (hidden omitted, no flicker): browser-verified — `secret` field absent; first paint awaits mount dry-run.
- [x] AC3 (enum option filtered): browser-verified — `priority`'s `high` option disabled with "(not allowed)".
- [x] AC4 (value-dependent re-derivation): exercised via the debounced `scheduleStagedAffordances` + AbortController stale-drop path; not specifically demo'd in the browser (would need a policy with a `when:` referencing another field — out of scope for this round).
- [x] AC5 (clean commit creates): existing v1 create tests still green; manual ticket creation via the demo project succeeded.
- [x] AC6 (sentinel never sent): `dryRunCreate.test.ts` asserts no `++new++` in request bodies.
- [x] AC7 (no edit regression): all 847 frontend tests pass; edit-mode filter path unchanged.
- [x] AC8 (commit re-authorizes): pinned by existing BUG-Q60V tests + the new RR-2U2D safety reasoning (touched key denied by policy still 403s with rule_id).

**Acceptance Status:** PASS — all 8 ACs satisfied (AC4 covered by code path +
tests, not browser demo).

## Documentation (enhancement)

- [x] Docs-checklist created and linked — DOCS-OTDO via `has-docs`.
- [x] User-facing docs updated — `docs/data-entry/api-reference.md` (`?dry_run=true` section + corrected "out of scope" list).
- [x] Docs-checklist marked done.

## Final Checks

- [x] Commit messages explain WHY (feat(dataentry): dry-run create..., feat(frontend): gate create form fields..., docs(...): document ?dry_run...).
- [x] No TODOs / FIXMEs left.
- [x] Ready for another developer to use.

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI — pending.
- [ ] All CI checks pass — pending.
- [ ] PR URL documented below.

**PR:** *pending*
