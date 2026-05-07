---
id: RR-IYG87
type: review-response
title: Handler bypasses Workspace.UpdateEntity — silently skips entity-update automations
finding: |-
    Pre-existing handler called a.ws.UpdateEntity(entity, oldEntity) which runs s.automation.Process(EventEntityUpdated, ...). The new handler stages writes directly via tx.WriteEntity and DOES NOT invoke the automation engine. CLAUDE.md documents server-side automations triggering on entity property changes (planning-checklist auto-creation, status side-effects). On develop, PATCH `status: ready` triggers create-checklist-on-ready. After this commit, it does not.

    Not in plan, not in commit message. Silent regression that breaks every workflow built on top of dataentry automations.

    Fix: wire the automation engine inside the WithTx callback after the entity write is staged. Or document explicitly + create follow-up. (a) is the right answer.
severity: significant
resolution: Added Tx.RunEntityUpdateAutomation which runs synchronous automation hooks (property-set actions) inside the transaction; return value is held by the handler and Workspace.ApplyAutomationSideEffectsAfterCommit is called after WithTx commits successfully to run side effects (create_relation, create_entity, Lua) outside the tx so a side-effect failure doesn't roll back the primary commit. Test TestV1Patch_AutomationFiresOnPropertyChange verifies a status-change automation sets completed_at on the entity.
status: addressed
---
