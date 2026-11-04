---
id: RR-B8SHAMZ
type: review-response
title: Cancel decision can leave focus on nothing
finding: 'RlChannelComposer.vue cancelCompose: focusField returns early for a type with no fields, and the cancel button is gone.'
severity: minor
resolution: After focusField, if the focus is not in the form it goes to the picker's tab stop, else the Send button (new class rl-channel-composer__send). Story CancelToNoFields.
status: addressed
---
