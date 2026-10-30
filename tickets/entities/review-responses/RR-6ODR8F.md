---
id: RR-6ODR8F
type: review-response
title: Refused column move is announced as done
finding: 'ListView.vue:81 moveColumn drops a move that would put a busy column first, but RlSortableList.vue:138 has already announced sortDropped for the new position before it emits move (and the drag path at :272 shows the preview order). A screen reader user hears that the row moved to position 1 when nothing changed, and a mouse user sees the row snap back with no reason given; the Columns description is the only hint. Fix: announce the refusal (a role=status message in ListView, such as "A busy column cannot be the first column"), or give RlSortableList a per-move veto so it announces the refusal itself. Add an assertion for the message to ListView.test.ts "move to the top is refused".'
severity: minor
resolution: ADDRESSED RlSortableList gained a refuseMove veto prop, a refuse event and a sortRefused message (en, nl); a vetoed keyboard or drag move announces the reason instead of the drop and shows no drop indicator; ListView passes refuseColumnMove and shows the reason in a warning RlCallout (data-testid config-column-refused) until the next move; ListView.test.ts drives the keyboard and asserts the status text and the callout; new RefusedMove story with a play function.
status: addressed
---
