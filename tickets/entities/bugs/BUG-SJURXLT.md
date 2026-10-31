---
id: BUG-SJURXLT
type: bug
title: Autosave misjudges its own state across saves of one tab
description: An unsaved value is replaced on screen while its error stays; a tab reports a body conflict against its own merged write; a second 412 in one save merges against the wrong base.
priority: high
why1: A later answer applied the server value over a field whose save had failed or conflicted; a merged body written while the user typed left the old base; a second 412 in one save merged against the save's first base.
why2: holdsProp/holdsContent only counted pending, debouncing and queued saves; nothing recorded that a field's last save did not go through or that the written body differed from the editor text.
why3: The base rules were written and tested one save at a time; the cases need two saves of one tab interleaved with another tab's writes.
why4: Unit tests pin one answer order each and only for one client; no test explored orders across two clients with faults.
why5: Autosave's state machine had no executable statement of its user-level guarantees (no blind overwrite, no silent loss, no false conflict), so each rule was checked only by the scenario that motivated it.
prevention: useAutoSave.twoClients.prop.test.ts states the guarantees as properties over two tabs, the verified fake and fc.scheduler(), with failing, empty-412 and lost-answer faults; mutations of each base rule make it fail.
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

## Symptom

Found by the two-tab autosave property test (TKT-V270TMQ). Three ways autosave
misjudges its own state across saves of one tab:

1. **Unsaved value replaced on screen.** A field whose save was refused as
a conflict, or failed with a transient error, keeps the user's value and shows
the error. The answer to a later save of another field then puts the server
value over it, while the error ("Your change was not saved. Edit it again ...")
stays. The user's value is gone.
2. **False body conflict with the tab's own write.** A body save that was
merged with another tab's change is written while the user keeps typing. The
next save merges against the old base and reports a conflict with the lines this
tab wrote itself.
3. **False conflict on a second 412 in one save.** When the merged resend is
refused again, the second merge runs against the save's original base, so the
lines merged in the first round read as a conflict.

## Fix

1. A field whose last save did not go through counts as held
(`unsavedProps`, `contentUnsaved`) until the user edits or reverts it.
2. A body written in a form the editor does not show yet is remembered
(`contentRebase`); the next save rebases the editor text onto it.
3. After a clean body merge, the save's merge base becomes the body it
merged with.
