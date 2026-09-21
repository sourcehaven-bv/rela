---
id: RR-PICKTY
type: review-response
title: Prefilled peers never seed pickerTypes, so a create can abort entirely
finding: '"applyEmbeddedPrefill writes relations into relations.value via applyTemplate but never populates pickerTypes. At submit, reshapeLegacyToModern returns null for any id it cannot type and handleSubmit aborts the WHOLE save with ''Some related entities have unknown types. Save aborted; reload the form and try again.'' This is the exact bug TKT-R4BMJM fixed for the link_as=to path, which carries a 20-line comment and seeds pickerTypes explicitly (DynamicForm.vue:794-816); the duplicate prefill reintroduces it. It usually survives because RelationPicker re-emits types for pre-existing selections, but fails when the peer is past the 50-page candidate cap (BUG-HOB9BR), when the peer is visible as a relation neighbour but not in the candidate list, or — most reachably — when the relation has NO field on the create form at all, since buildDuplicatePrefill filters by user selection rather than by what the form renders. resolveOutOfPageLinks returns early in create mode, so the normal backstop is absent. The advice in the error (''reload the form'') cannot fix it."'
severity: critical
resolution: 'applyEmbeddedPrefill now seeds pickerTypes from the peer type the prefill already carries, mirroring the link_as: to path. A duplicate is more exposed than a pre-link because it carries relation types the create form may not render at all. Covered by ''seeds picker types so a non-card relation does not abort the create''.'
status: addressed
---

## Suggested resolution

Seed pickerTypes from the prefill. The peer type is already carried on DuplicatePeer, so this is a few lines.
