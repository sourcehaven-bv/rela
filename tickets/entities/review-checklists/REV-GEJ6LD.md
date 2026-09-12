---
id: REV-GEJ6LD
type: review-checklist
title: 'Review: flatten interpolated values rather than the operator''s template'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just test` green, no FAIL lines. `just lint` reports 0 issues. `just
comment-lint` reports no unresolvable doc links across 14034 comments. `just
coverage-check` passes both thresholds: package 50% and total 65% satisfied, at
79.6% total (40083/50327). `just arch-lint` OK, no warnings. `just lint-md` 0
issues across 270 files. `just docs-check` green, so the prose in
`docs/webhooks.md` survives a regeneration run.

No new comment findings were introduced; nothing needed suppressing.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

The reviewer returned no critical findings and stated the asymmetry holds: every
producer-controlled string reaches the document through one seam, the
substitution write in `interpolate`, and moving the flattening per-value is
strictly stronger than what it replaced, because the old code flattened only the
`append_section` path and left `set:`, `find:` values and `create_if_missing:`
properties unflattened entirely.

Three bypasses were checked and are closed: no recursive interpolation (a value
of `{{body.b}}` renders literally), no adjacent-substitution splice (`x="{{"`
and `y="body.b}}"` renders literally), and no raw newline via composite values
(objects and arrays go through `json.Marshal`, so the newline is already
escaped).

Three significant findings were raised and all three are fixed in this diff. The
first was a real latent bug rather than a naming quibble.

**Review Responses:** RR-8KQ2MW (significant, addressed), RR-4TN7PD
(significant, addressed), RR-6VX1JC (significant, addressed), RR-2HD5RQ (minor,
addressed), RR-9WGT3B (nit, addressed), RR-5PLN0S (minor, addressed), RR-3JBQ7Y
(minor, deferred with reason).

Self-review: the diff touches `internal/dataentry/webhook_routes.go` and its
test, `internal/markdown/section.go` and its test, and `docs/webhooks.md`. The
`internal/markdown` change is not unrelated scope — it is the fix for RR-8KQ2MW,
which exists because this change is what first passes a multi-line block to that
function. Two probe files the reviewing agent left in the working tree were
deleted before committing; `git status` is clean apart from the ticket entities.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. Operator multi-line `content:` survives — PASS.
`TestWebhookRoutes_AppendSectionKeepsOperatorNewlines` (end to end) and the
`operator newline kept` row of
`TestWebhookPayloadInterpolate_FlattensEveryValue` (at the seam).
2. Interpolated-value newline still flattened — PASS. The same end-to-end test
asserts `- detail: one two` from a value of `one\ntwo`, and the seam test covers
`\n`, `\r` and NUL across the body, query and header namespaces.
3. Injection test unchanged and passing — PASS.
`TestWebhookRoutes_AppendSectionFlattensNewlines` is byte-identical to the
pre-change version and still asserts no sibling heading is planted.
4. Flattened, never dropped — PASS. Asserted in both the injection test and
`TestWebhookPayloadInterpolate_ValueCannotForgeAHeading`.

Mutation evidence that the tests are not vacuous: removing the `flattenToLine`
call from `interpolate` and re-running makes the injection test fail with `##
Injected` rendered as a real heading, and fails both new seam tests. Restoring
it makes them pass.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-HUD0HY

## Final Checks

- [x] Commit message explains the why, not just what
