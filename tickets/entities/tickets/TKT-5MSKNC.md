---
id: TKT-5MSKNC
type: ticket
title: Editor e2e specs are flaky under parallel load
kind: test
priority: low
effort: s
status: done
---

## Problem

Under `--repeat-each=3` the `tests/markdown-editor*` specs fail 1-2 runs out of
~70, with a **different test each time**. Every one passes in isolation.

## This is pre-existing, and that was verified

Found while implementing TKT-6MZ42J. To rule that change out, the working tree
was stashed and the baseline measured the same way: **the baseline fails 2 of
69**, versus 1 of 71 with the change applied. Not a regression.

## The original hypothesis was WRONG

This ticket was filed blaming `FormPage.typeIntoEditor` for typing before
ProseMirror had focus, so the tail of the string was dropped. **Measured on
`develop` at `--repeat-each=8`: that is not what happens.** Every failing
snapshot showed the text typed *completely* — `see @mentionzzn4zn2`, not `see`.
The dropped-keystroke theory came from one snapshot during TKT-6MZ42J that had a
different cause, and it was recorded here without being re-checked.

## The real cause: a section-ambiguous locator

`mentionMenuOptions` matches `.mention-menu-item` in **both** sections of the
menu — the type picker AND the entity results. Three tests gated on
`mentionMenuOptions.first()` being visible and then pressed Enter:

- `Enter inserts the reference and closes the menu`
- `the inserted reference renders as a titled link inside the editor`
- `round-trip: the saved body renders the rewritten link on the detail page`

Under parallel load the entity search is slow, so the first visible row is a
**type** row. Enter on a type row *scopes the search* instead of inserting, so
the query stays plain text and no `entityRef` node is ever created — the
assertion then fails on a missing `a[data-entity-ref]`. A failing snapshot
confirms the mechanism: it shows a `bug` scope chip set and "No matches", which
is only reachable if Enter selected a type.

This is a defect in the tests introduced by TKT-6MZ42J (the type section is
new), not in the product. The three tests were asserting "a row is visible" when
they meant "an entity row is visible".

## Fix

Gate those three on `mentionMenuEntityOptions.first()`, which is scoped to
`ul[data-section="entities"]`, and raise the timeout to 10s so a slow search
waits rather than racing. `typeIntoEditor` is left alone — the evidence does not
support changing it.

## Result

`--repeat-each=8` (232 runs): **4 failed → 1 failed**, and the three
mention-insert failures are gone. `--repeat-each=3` three times over: **261/261
passed**, where the same command previously failed 1-2 per run.

## Remaining, both infrastructure not logic — out of scope here

At `--repeat-each=8` (8x parallel load, well above CI's), two non-menu failures
remain and are timeouts rather than wrong behaviour:

1. `openEntityPicker` — the toolbar button resolves but does not become
click-stable within Playwright's 5s default.
2. A `beforeEach` fixture `POST /api/v1/features` exceeding the 5s API timeout.

Both are contention on an overloaded machine. Worth a separate look at whether
the e2e worker count and these timeouts are matched, but neither is a product
bug and neither reproduces at the repeat level this ticket was filed about.

## Why it mattered

A suite that fails ~2% per run trains everyone to re-run rather than read the
failure. TKT-6MZ42J hit this directly in both directions: a genuine product bug
was first dismissed as "probably the flake", and a genuine flake was first
investigated as a product bug. This ticket is itself a third instance — the
filed cause was a guess that survived into the record unverified.
