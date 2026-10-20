---
id: RR-WR80AA
type: review-response
title: Edit end does not restore the type the user had selected
finding: 'RlChannelComposer.vue:141-159. The stash saves the draft of the edited message''s type but not the selected type before the edit. Example: a user is writing a Decision, edits an old Message, and cancels. The composer now shows the Message type, and the half-written Decision is hidden behind the segmented control. Fix: stash `selected.value` as well and restore it when editing ends.'
severity: minor
resolution: The stash now saves and restores the selected type as well as the edited type's draft and refs.
status: addressed
---
