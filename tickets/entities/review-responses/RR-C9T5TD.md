---
id: RR-C9T5TD
type: review-response
title: Show-all toggle has no visible pressed state
finding: 'RlChannel.vue:450 renders RlButton variant=ghost with :aria-pressed and a fixed label ''Show all messages''. RlButton.vue has no [aria-pressed=''true''] style, so a sighted user sees an identical button in both states, and the label invites them to press it again to show all messages after everything is shown. Only screen readers get the state. When the loaded page has no quiet message, a press changes nothing visible at all. Fix: add a pressed style to RlButton (for example .rl-button[aria-pressed=''true''] with the selected background token) so every toggle in the library gets it. Assert it in the ''Quiet types'' story with a computed-style check, or by reading the source as styles/focusRing.test.ts does.'
severity: significant
resolution: Fixed. RlButton gives [aria-pressed='true'] the selected fill and an inset accent edge in every variant, also on hover, so every toggle in the library shows its state. New story Common/Button "Toggle" checks the computed box-shadow and background before and after a press.
status: addressed
---
