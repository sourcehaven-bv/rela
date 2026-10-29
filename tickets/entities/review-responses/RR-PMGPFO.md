---
id: RR-PMGPFO
type: review-response
title: tone is not a missed shared type; align with the in-file precedent instead
finding: 'archplan concepts.string_enum: ChannelDecisionView.tone (''default''|''danger'', types.ts:321) is RlButton''s/RlIconButton''s/RlMenuItem''s/RlConfirmDialog''s button tone. It is not MessageTone (''info''|''success''|''warning''|''danger''), which is a feedback concept. Introducing a shared ButtonTone across four common components is a separate refactor and does not belong in this diff. The more relevant inconsistency is in the same file: ChannelActionView uses `danger?: boolean` (types.ts:72-73) for the same idea. Consider `danger?: boolean` here, which also drops a ''default'' value that carries nothing, or leave as is and file the ButtonTone extraction separately. archplan types.synonyms PostMessageRequest(apiwire)/PostInput(realtime) is not a defect: one is the JSON wire DTO, the other the service input with transport-only LogOnly, and the handler maps them explicitly (channels_handler.go:409). Merging them would leak wire tags into realtime or LogOnly onto the wire.'
severity: nit
resolution: 'ChannelDecisionView now uses danger?: boolean like ChannelActionView, which removes the tone string enum; the two archplan findings need no code change and are left for the baseline.'
status: addressed
---
