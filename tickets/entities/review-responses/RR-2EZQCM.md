---
id: RR-2EZQCM
type: review-response
title: Preset is merged at Send even when the composer is not in compose mode
finding: 'useSessionDecisions.ts:259 merges f.compose.preset whenever formDecision is of the posted type. The composer can stop applying compose on its own (droppedKey at RlChannelComposer.vue:273-279, or an edit, line 258), and then it renders the given properties as normal editable fields. Any desync between the app''s record and the composer''s view then overwrites user input with the preset without telling the user. The plan calls this one source of truth, but in practice there are two (formDecision and droppedKey). Fix: carry the compose key in ChannelPostEvent (or emit cancelCompose when the composer drops it locally) and merge the preset only when the event''s key matches the record.'
severity: minor
resolution: "addressed\tChannelPostEvent now carries the compose key the composer applied, and OpenChannel passes it to sendComposer. The preset is merged only when that key matches the record. Tests cover a mismatched key and a missing key: neither Send carries the preset, and both keep the generation."
status: addressed
---
