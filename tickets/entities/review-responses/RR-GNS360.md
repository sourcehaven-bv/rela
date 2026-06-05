---
id: RR-GNS360
type: review-response
title: SSE bridge maps only entity events; relations emit events nobody broadcasts, attachments emit none
finding: 'Design-review verification: (1) Relation writes emit EventRelationCreated/Updated/Deleted from the store, but NO dataentry code broadcasts relation changes to SSE today (api_v1 relation handlers don''t call broadcast*). The PATCH handler explicitly does NOT broadcast on relation-only changes (api_v1.go:903 comment ''Relation-only changes don''t fire an entity:updated event today''). (2) AttachFile/DeleteAttachment emit NO store events at all (attachment.go) — so attachments can''t be bridged even if we wanted to. So the plan''s ''bridge store entity events to SSE'' leaves relation changes from a REMOTE process invisible to the browser, and there''s no parity question for attachments (no events exist). This is a scope/correctness gap: cross-process live-reload would work for entities but not relations.'
severity: significant
status: open
---

## Resolution (plan update)

Decide the live-reload fidelity explicitly:
- **Entities:** bridge EventEntityCreated/Updated/Deleted ->
broadcastEntityEvent (matches today's local behavior exactly).
- **Relations:** today relation edits trigger NO SSE broadcast even locally, so
cross-process relations are no WORSE than the status quo. Two choices: (a) match
status quo — don't bridge relation events (relation live-reload is simply not a
feature, locally or remotely); document it. OR (b) IMPROVE — bridge
EventRelation* to a generic `broadcast("refresh")` so a remote relation edit
nudges the browser to refetch. Leaning (a) for this ticket (keep scope to
matching local behavior cross-process; relation live-reload is a separate
enhancement), but call it out in AC so it's a conscious decision, not an
oversight.
- **Attachments:** the store emits no events; out of scope, no change.

Update AC4 to state: cross-process SSE parity is for ENTITY create/update/delete
(the only thing broadcast locally today). Relations/attachments explicitly not
in the live feed (unchanged from current behavior). This keeps the de-dup
mapping exact: bridge entity ops only; the 3 inline entity broadcasts are
replaced 1:1.
