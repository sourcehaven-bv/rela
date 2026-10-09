---
id: RR-OVRN8W
type: review-response
title: 'Design: rename drops the pending job'
finding: The payload names the old id and rename runs no automations.
severity: significant
resolution: New on.updated trigger fires on any change and on EventEntityRenamed; the job service rides the AliasRewriter fanout and re-triggers updated automations for the new id. TestBackgroundAction_RenameRetriggers.
status: addressed
---
