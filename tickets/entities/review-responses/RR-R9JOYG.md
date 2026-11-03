---
id: RR-R9JOYG
type: review-response
title: Stepper summary and composer hint concatenate translated fragments
finding: 'RlChannelSession.vue:56-68 builds "Stranded · x", "x · Paused" and "Session · x" from separate keys with a hard-coded " · " and fixed order. The template adds ", " before sessionNext and sessionEndState (lines 110-111). RlChannelComposer.vue:153 wraps sessionMovesSession in a hard-coded " (...)". A locale that needs another order or other punctuation cannot override this. Fix: add Messages functions (for example channelSessionSummary({status, label}) and channelSessionOptionHint()) and render their result whole.'
severity: nit
resolution: channelSessionSummary, channelSessionStep and channelSessionOptionHint render whole templates; the fragment keys are removed; a LocalizedTexts story checks the override.
status: addressed
---
