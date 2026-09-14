---
id: RR-N8CNLQ
type: review-response
title: Escape did not close the @ mention menu
finding: The keydown handler called menu.close() on Escape, but SlashProvider re-evaluates shouldShow on every editor update and re-opened the menu immediately, because the query still parsed. Caught by an e2e test that had never been run ('Escape closes the menu without inserting').
severity: significant
resolution: Escape records the dismissed query and hides the provider. shouldShow refuses while the query is unchanged and clears the latch as soon as the user edits it, so the menu returns on the next keystroke rather than being latched off for the session. activeMatchLength is cleared on every close, and insertRef refuses when there is no live query rather than deleting whatever precedes the cursor.
status: addressed
---

## Finding

`menu.close()` sets the component's own state, but visibility belongs to
`SlashProvider`, which asks `shouldShow` on every update. The query still
parsed, so it answered true and the menu reappeared before the user saw it
close.

## Resolution

Remembering *which* query was dismissed, rather than latching the trigger off:

- Escape records `dismissedQuery` and calls `slashProvider.hide()`
- `shouldShow` returns false while the query still matches the dismissed one
- any edit to the query clears the latch, so the menu comes back

Latching the trigger off entirely would have been the easy fix and the wrong
one: the user would have had to delete the `@` and start over.

## Related, from the same area

The code review noted `activeMatchLength` is never reset on close, and that
adding a second piece of separately-reset menu state made that riskier. It is
the span `insertRef` deletes, so a stale value could eat characters that are no
longer part of a query. It is now cleared on every close, and `insertRef`
refuses outright when there is no live query.
