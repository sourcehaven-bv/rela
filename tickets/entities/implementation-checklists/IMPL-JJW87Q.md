---
id: IMPL-JJW87Q
type: implementation-checklist
title: 'Implementation: Make searchVisibleHits fail closed when the searcher cannot redact fields'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — two in
`internal/dataentry/search_fail_closed_test.go`.
- [x] Integration tests written — they drive `searchVisibleHits` with a searcher
of the exact shape that causes the bug (a `VisibleSearcher` that is not a
`FieldVisibleSearcher`), rather than asserting on an internal flag.
- [x] Happy path implemented
- [x] Edge cases from planning handled — both orders of (policy hides?, searcher
can redact?) are covered.
- [x] Error handling in place — yields `search.ErrScope`, the sentinel the caller
already maps to `errACLListQuery`.

## Test Quality

- [x] Using fixture builders or factories — two small purpose-built doubles; the
package's app fixtures would drag in a whole App for a two-collaborator free
function.
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter — each double implements exactly the
method set that makes the type assertion hit or miss.
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*Mutation-verified.* Restoring the fall-through:

```
--- FAIL: TestSearchVisibleHits_FailsClosedWithoutFieldSearcher
    want an error when the searcher cannot redact; got 1 hit(s) and no error
```

The message states the failure in the terms that matter — a hit was served —
rather than "expected error, got nil".

*The test asserts more than "an error came back".* `entityOnlySearcher` records
whether it served at all, and the test fails if it did. An implementation that
yielded an error *after* the fallback had already produced hits would satisfy a
naive error assertion while still opening the oracle; here the fallback must not
run at all.

*The negative case is load-bearing.*
`TestSearchVisibleHits_NoRedactionNeededStillWorks` uses the same non-redacting
searcher with the Nop resolver and asserts the hit **is** served. Without it,
"fail closed whenever the searcher isn't a FieldVisibleSearcher" would also pass
— and would break the default deployment, where the Nop resolver makes redaction
a provable no-op.

**Gates:** `go test ./...` exit 0; `just lint` 0 issues; arch-lint,
comment-lint, plimsoll, lint-md clean.

## Quality

- [x] Code follows project patterns — same sentinel (`search.ErrScope`), same
fail-closed shape and same rationale as `search.Visible.SearchVisibleFields` one
layer down.
- [x] Checked for DRY opportunities — the two conditions were **split** rather
than combined. They looked like one check (`!ok || !hides`) and are two
different facts: one is a wiring bug, the other is a legitimate no-op.
Collapsing them is what allowed the bug.
- [x] No security issues introduced — the change can only refuse where it
previously served. The error names the searcher's Go type, not policy content or
entity data: a wiring diagnostic, not a description of what was hidden.
- [x] No silent failures — that is the entire ticket.
- [x] No debug code left behind.

**A confidently-wrong comment was corrected.** The godoc already *claimed* this
guarantee — *"When redaction IS in play but the searcher can't do it,
SearchVisibleFields fails closed"* — while the code returned before ever
reaching `SearchVisibleFields`. The doc described the intended behaviour of a
method that was never called. It now describes what this function does, and
cites RR-8W40EW for why.
