---
id: RR-3CWLS3
type: review-response
title: AC4 assumed a capped cascade fails the delivery
finding: A refused cascade write lands in AutomationErrors; CreateEntity/PatchEntity still succeed and ingest ignored the errors, so the hook answered 200 and the refused write vanished.
severity: significant
resolution: AC4 rewritten to the actual behaviour (write stands, cascade refused). ingest now logs AutomationErrors at warn level after create and patch. docs/webhooks.md states it.
status: addressed
---
