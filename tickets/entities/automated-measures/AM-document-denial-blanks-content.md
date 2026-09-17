---
id: AM-document-denial-blanks-content
type: automated-measure
title: A denied document render clears the content on screen, while a transient failure keeps it
description: Unit tests for the shouldDropHeldContent classifier plus component tests, in BOTH document loaders, asserting the rendered body is cleared on 401/403/404 and kept on every other failure.
kind: test
location: frontend/src/api/errors.test.ts, frontend/src/views/DocumentView.rerender.test.ts, frontend/src/components/entity/DocumentsPanel.denial.test.ts
status: active
---

## Measure

Three files, because the rule has parts that fail independently.

1. **The classifier** (`frontend/src/api/errors.test.ts`). `shouldDropHeldContent`
is true for 401, 403 and 404 and false for 400, 422, 500, 503, a network
failure, a cancellation, and any non-`ApiError` rejection. Table driven, so
adding a status means adding a row, not a branch.

The fixture builds each case with the status inside the `ProblemDetail` body as
well as on the response, because `normalizeApiError` prefers the body's
`status`. A fixture pinned at one status silently tests the same case every time
— the first draft did exactly that and three tests failed for the fixture's
reasons rather than the code's.

2. **DocumentView** (`DocumentView.rerender.test.ts`). An SSE re-render
rejected with each denial status leaves no `.document-body` in the DOM and shows
the empty state. A fourth test covers the generation fence: a denial from a
*superseded* render must NOT blank, since it concerns a document no longer
displayed and would erase a body the newest render was allowed to paint.

3. **DocumentsPanel** (`DocumentsPanel.denial.test.ts`). The same three denial
cases against the second loader. The shared classifier stops the two components
*disagreeing* about what a denial is, but the decision to act on it is still a
separate line in each file — an untested copy is deleted by the next refactor
with CI green.

## Why this shape

The denial case and the transient case reach the same catch block and differ
only in the error they carry, so a test for one proves nothing about the other —
which is how the original regression shipped green. Both suites therefore keep
the transient-failure test (content SURVIVES a generic failure) directly beside
the denial tests. Deleting either leaves the remaining rule satisfiable by code
that ignores the distinction entirely.

`DocumentsPanel.denial.test.ts` also opens with a test that the panel renders at
all. Every other assertion in that file is about content *disappearing*, so a
harness that silently mounted nothing would pass them all.

## What this measure does NOT cover

The `isCached` flag is not asserted, and deliberately so. The cached badge
renders inside the `v-else-if="docContent"` branch that blanking already
unmounts, and every successful render reassigns the flag from its response, so a
stale `true` has no path to the screen. An earlier revision of this entity
claimed a mutation-verified assertion here; that claim was false — removing an
`isCached.value = false` line left the whole suite green, because the test could
only observe a badge that had already been unmounted for an unrelated reason.
The line was removed rather than papered over with a test that cannot fail.

## Mutation-verified

Measured against the 52 tests in the three files, baseline green:

| Mutation | Failures |
|---|---|
| Remove the branch from `DocumentView.loadDocument` | 3 |
| Remove the branch from `DocumentsPanel.loadDocument` | 3 |
| Narrow the classifier to 403 only | 6 |
| Widen the classifier to include 500 | 1 |
| Drop the classifier's `instanceof` guard | 3 |
| Hoist the blanking above the generation fence | 1 |

The transient-failure tests pass under every one of these, confirming they do
not mask the denial assertions.
