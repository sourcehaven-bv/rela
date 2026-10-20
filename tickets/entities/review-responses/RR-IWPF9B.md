---
id: RR-IWPF9B
type: review-response
title: Edit of a message whose type vanished leaves no way out
finding: 'RlChannelComposer.vue:83-90, 231. In edit mode `offered` holds only the edited type. If a descriptor reload removed that type, the form does not render (`v-if="current"`), so there is no Cancel button and the app stays in edit state with no visible composer. Fix: when editing and the type is unknown, render a small notice with a Cancel button that emits cancelEdit, or emit cancelEdit from a watcher.'
severity: minor
resolution: An edit of a type the descriptor no longer has now renders a notice with a Cancel button that emits cancelEdit (story EditTypeGone).
status: addressed
---
