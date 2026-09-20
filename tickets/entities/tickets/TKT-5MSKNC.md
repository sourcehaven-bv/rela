---
id: TKT-5MSKNC
type: ticket
title: Editor e2e specs are flaky under parallel load
kind: test
priority: low
effort: s
status: backlog
---

## Problem

Under `--repeat-each=3` the `tests/markdown-editor*` specs fail 1-2 runs out of
~70, with a **different test each time**. Observed failures include "can fill
body content and submit form", "Escape closes the picker without inserting",
"the inserted reference renders as a titled link", and "round-trip: the saved
body renders the rewritten link".

Every one of them passes in isolation and passes on a single non-repeated run.

## This is pre-existing, and that was verified

Found while implementing TKT-6MZ42J. To rule that change out, the working tree
was stashed and the baseline measured the same way: **the baseline fails 2 of
69**, versus 1 of 71 with the change applied. Different tests, same rate. Not a
regression.

## Likely cause

`FormPage.typeIntoEditor` clicks the ProseMirror surface and types immediately,
with no wait for focus to settle:

```ts
async typeIntoEditor(text: string): Promise<void> {
  await this.proseMirror.click()
  await this.page.keyboard.type(text)
}
```

Under parallel load a keystroke can land before ProseMirror is listening, so the
tail of the string is dropped and a later assertion fails on a half-typed query.
A failing snapshot from TKT-6MZ42J showed the document holding `see` when `see
@featur` had been typed.

`expectEditorText(text)` was added in that ticket as a gate for exactly this,
and the two tests that use it are stable. The fix is probably to make
`typeIntoEditor` itself verify what it typed, rather than leaving it to each
caller to remember.

## Why it matters

A suite that fails ~2% of the time per run trains everyone to re-run rather than
read the failure, which is how a real regression gets waved through. The
TKT-6MZ42J work hit this directly: a genuine product bug was initially dismissed
as "probably the flake", and a genuine flake was initially investigated as a
product bug.

## Out of scope

Fixing the editor's focus handling itself — the evidence points at the test
helper, not the component.
