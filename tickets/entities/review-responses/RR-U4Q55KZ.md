---
id: RR-U4Q55KZ
type: review-response
title: A step refused by another run's outside_edits returns 500
finding: FlowControl.Complete passed the FieldWriteError from another run's outside_edits through classify unmapped, so the transports answered 500.
severity: significant
resolution: classify maps an affordances.FieldWriteError to flowview.ErrForbidden (403). TestOutsideEdits_OtherRunStillNarrows asserts ErrorIs ErrForbidden.
status: addressed
---
