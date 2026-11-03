---
id: RR-WOA2V8
type: review-response
title: Composer chip in a ref field is a dead button
finding: 'RlChannelField.vue:148. RlRefChip without href renders a button that emits `open`, and nobody listens in the composer. Keyboard users tab to a control that does nothing. Fix: add a non-interactive mode to RlRefChip (render a span when neither href nor a listener is given), or pass `inert`.'
severity: nit
resolution: RlRefChip gained a plain mode that renders a span, and the composer uses it for the chosen ref.
status: addressed
---
