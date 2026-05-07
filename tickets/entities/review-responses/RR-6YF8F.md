---
id: RR-6YF8F
type: review-response
title: 'data: [] footgun for auto-save — needs defensive measure or clear documentation'
finding: |-
    TKT-18JS6's auto-save composable will be the primary client. Likely builds PATCH body from form state. If form widget defaults to {data: []} on mount and only populates after fetch, first auto-save fire could land BEFORE fetch completes, sending relations.tagged.data: [] and wiping every tagged relation. Not theoretical — auto-save has fewer human checkpoints by design.

    Fix: pick one defensive approach:
    - (a) Server-side opt-in: relations_strict: true flag (default false). When false, data: [] requires also setting confirm_replace_relations: ['tagged', ...] allowlist or 422.
    - (b) Client-side discipline: auto-save composable holds off on sending relations keys until fetch completes. Documented contract.

    At MINIMUM: API reference must have a flashing-red callout: 'Sending data: [] deletes all edges of that relation type. Ensure form state has been fetched before first save when building bodies via spread.'

    Recommend (b) + the callout. Wire-format simpler.
severity: significant
resolution: 'Per user direction (Option 3b): client discipline + documentation callout. Auto-save composable in TKT-18JS6 must guard against sending relations keys before fetch completes. API reference includes a flashing callout. No server-side flag added — keeps wire format simpler.'
status: addressed
---
