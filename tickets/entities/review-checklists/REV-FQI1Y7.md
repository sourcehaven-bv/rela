---
id: REV-FQI1Y7
type: review-checklist
title: 'Review: Autosave conflict resolution: per-field version preconditions, three-way merge on 412, bounded auto-retry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`; run through `just coverage-check`, exit 0; frontend `npm run test:run` 3233 passed before the last test additions, composables suite 470 passed after)
- [x] Lint clean (`just lint`: 0 issues; eslint clean on touched files; `just lint-md` 0 issues; plimsoll and arch-lint OK)
- [x] Comment lint gate clean (`just comment-lint`: no unresolvable doc links)
- [x] Coverage maintained (`just coverage-check`: total 80.4%, all floors pass)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer in parallel)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-8K0AEM, RR-MZO79B, RR-H7762P, RR-QZZIMN, RR-WF3MB6 addressed; RR-N8UUS1 deferred by the user to TKT-E9WXPU)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Security: RR-8K0AEM (addressed). Significant: RR-MZO79B,
RR-H7762P, RR-QZZIMN, RR-WF3MB6 (addressed); RR-N8UUS1 (deferred, TKT-E9WXPU).
Minor: RR-XNKLVU, RR-75C920, RR-PLR7BP (addressed); RR-0SMGVB, RR-SYJ30H,
RR-QRSO51, RR-8L7L0B (deferred with reasons). Nits: RR-2H33BA, RR-Z6TSXU
(addressed); RR-E09J0I (wont-fix). Earlier design-review responses: RR-DBL90Y,
RR-GDE3PY, RR-P6ZFSV, RR-PP9UEF, RR-QSO6HF.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 versions exposed: PASS. TestV1GetEntity_Versions (redacted field has no token, unset field has an absent token); curl GET showed tokens for every declared property plus content and relations.
- AC2 preconditions sent for exactly the written keys: PASS. 'sends a precondition for exactly the field it writes'; TestValidatePreconditionScope and the "unwritten field → 400" subtest; curl returned 400 invalid_precondition.
- AC3 disjoint fields succeed first time: PASS. "other field" subtest of TestV1UpdateEntity_Preconditions; browser: a priority edit on a stale page saved with one request while another client had changed status and description.
- AC4 same-field conflict surfaces: PASS. 'reports a real conflict, writes nothing, and keeps the user value on screen'; browser: inline conflict message, server kept the other value, form kept the user's.
- AC5 disjoint body hunks merge: PASS. 'merges a body edited on different lines and resends with the fresh token'; mergeText table tests.
- AC6 no markers, no write on conflict: PASS. 'never emits conflict markers'; 'refuses a body edited on the same line' asserts one PATCH only.
- AC7 retry bounded: PASS. 'gives up after three attempts and reports the error'; 'moves no base when the attempts run out'.
- AC8 no behavioural regression: PASS. Existing useAutoSave suites pass unchanged in behaviour; call-shape assertions assert the precondition payload positively.
- AC9 hidden-field churn does not block: PASS. Tokens are per field, so a concurrent write to another field leaves the written field's token unchanged ("other field" subtest; lost-race test "race on another field: empty conflicts").
- AC10 redacted property never unset: PASS. 'never unsets a property the refetch omits'.
- AC11 cross-channel writes do not collide: PASS. TestV1UpdateEntity_OtherChannelDoesNotCollide; 'treats a queued body save as unsaved when an earlier response lands'.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-0MW4OE

## Final Checks

- [x] Commit message explains the why, not just what (to be written at commit time, when the user asks)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the PR is opened after `done`, when the user asks; see TKT-UFV01M)
