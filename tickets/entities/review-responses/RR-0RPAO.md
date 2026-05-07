---
id: RR-0RPAO
type: review-response
title: SSE event story claims 'consolidates N+1 events' but those granular events don't exist
finding: |-
    Plan: 'Single entity:updated SSE event per successful PATCH (consolidates N+1 events that would have fired with separate calls).' watcher.go only has broadcastEntityEvent for create/update/delete on entities. There are NO relation:created/relation:deleted broadcasts in the data-entry server. The 'N+1 events' framing is misleading.

    Fix: rewrite the AC: 'A successful PATCH fires exactly one entity:updated SSE event, regardless of how many properties or relations changed. Test: subscribe a test broker, PATCH with multiple changes, assert exactly one event delivered.' Drop the N+1 parenthetical. Update Scope (OUT) entry that says 'Granular relation:created/relation:deleted events stay for compat' — they don't exist; nothing to deprecate.
severity: minor
resolution: 'Misleading ''N+1 events'' framing dropped. Plan now says ''single entity:updated event for the PATCHed entity, plus one per affected symmetric/inverse counterparty.'' AC #21 specifies the count exactly. No nonexistent relation:* events claimed.'
status: addressed
---
