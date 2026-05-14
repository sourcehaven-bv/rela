---
id: RR-E25Z
type: review-response
title: Cursor moves to the right on same line keep the session open
finding: 'useBacktickAutocomplete.ts `onCursorActivity` (lines 432-440) only closes the session when `cursor.line !== trig.line` OR `cursor.ch <= trig.ch`. If the user clicks elsewhere on the SAME line PAST the trigger range — e.g. trigger at ch:5, typed up to ch:10, then clicks at ch:30 to position cursor far past the typed text — the session stays open. The popup remains anchored at the trigger position, but `typedAfterTrigger()` (line 227-233) returns `getRange(trig+1, cursor)` which now includes ALL text between the trigger and the new cursor position, including arbitrary content the user moved past. Subsequent keystrokes feed garbage into `filterPrefixList` and `scheduleSearch`. This will produce "why is the popup still open and why are the suggestions nonsense?" bug reports. Fix: track the expected cursor position (last known end-of-typed-range) and close if the cursor moves past it via mouse click, not just to the left.'
severity: critical
resolution: Composable now tracks `expectedCursorCh` after every change. cursorActivity closes the session if the cursor moves past expectedCursorCh+1 (the +1 absorbs the change/cursorActivity ordering race during typing). New unit test 'closes when cursor jumps past the typed-after-trigger range' covers a mouse click far past the typed range.
status: addressed
---
