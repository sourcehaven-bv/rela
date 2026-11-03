---
id: RR-SY7QUQQ
type: review-response
title: Hidden Details tab marks channels read
finding: With Details kept mounted by v-show, an open channel in the above_relations or bottom block stays mounted while a channel tab is shown. useMarkRead.wanted() checks only isPageHidden() (useMarkRead.ts:57), so messages the user cannot see get marked read and the inbox unread count drops.
severity: significant
resolution: Deferred to TKT-B75I7LH with the tab design; the planned fix is recorded there.
reason: 'Applies only to placement: tab, which moved to TKT-B75I7LH (approved scope change: this ticket ships above_relations only).'
status: deferred
---
