---
id: RR-A92OSQ
type: review-response
title: aria-describedby target does not exist yet
finding: Approach 4 describes the row group by 'the author line's id'. RlChannelAuthor and the meta div have no id today (RlChannelMessage.vue ~165), so a useId is needed. For a card message, the type label (card head, for example 'plan') plus the author says more than the author alone; describe the group by both.
severity: nit
resolution: RlChannelMessage gets useId ids for the card head and author line, and the row group is described by both.
status: addressed
---
