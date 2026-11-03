---
id: RR-LUSV08
type: review-response
title: Side panels share traverse and sections unvalidated
finding: forms.*.side_panel reuses ViewTraverse and ViewSection but only spans are validated, so new keys there would be silently misapplied.
severity: significant
resolution: validateSidePanelMentions refuses mentioned_by and per-type keys in side panels. TestValidateConfig_MentionsRefusedInSidePanel.
status: addressed
---
