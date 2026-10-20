---
id: RR-2TTSW1
type: review-response
title: Edit save with an evicted message erases unrendered props
finding: 'frontend/src/components/entity/channels/OpenChannel.vue onSave (around the toEditProps call): held is looked up in ch.messages at save time, not at edit start. If the held list is replaced between onEdit and onSave (reload after a gap, loadNewest, target refetch), held is undefined, toEditProps (frontend/src/utils/channelValues.ts:199) carries nothing for fields the composer did not render, and because an edit replaces the props the server drops every hidden:/presentation-omitted non-ref value. Restricted refs survive because the server keeps omitted restricted refs, so this is an integrity loss on the author own message, not a disclosure. Fix: capture the message props in editingFor/onEdit and pass that snapshot to toEditProps, or refuse the save (clear editing and show an error) when held is undefined; add a test that saves after the held list is reloaded.'
severity: minor
resolution: 'Same fix as the code review edit finding: the PATCH is built from props snapshotted at edit start, and a save without a snapshot is refused with a form error.'
status: addressed
---
