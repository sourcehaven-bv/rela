---
id: RR-C77VUYU
type: review-response
title: Server instructions mention filtered tools
finding: serverInstructions still names watch, schema and update_entity when they are filtered.
severity: nit
reason: Instructions are sent once per session and describe conventions; a call to a filtered tool fails clearly. Not worth threading the filter into the text.
status: wont-fix
---
