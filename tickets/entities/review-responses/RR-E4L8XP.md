---
id: RR-E4L8XP
type: review-response
title: Edit SQL with an empty target set
finding: '`DELETE ... AND target NOT IN (…)` with an empty new set is a syntax error on Postgres, though SQLite accepts it. Branch to a plain `DELETE ... WHERE channel_id = ? AND seq = ? AND kind = ''mention''` when the set is empty, and add a channelstest row "edit that removes the only mention" (it is already in the edge cases).'
severity: nit
resolution: Empty new set uses a plain DELETE of the message's mention rows; channelstest row "edit removes the only mention".
status: addressed
---
