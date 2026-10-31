---
id: RR-7OBXDW
type: review-response
title: After a type switch and back, the visible prefill stays but the unrendered preset is dropped
finding: '`onComposerSelect` ends the form decision on any type switch (useSessionDecisions.ts:268-271). AC 10 makes `formDecision.preset` the only carrier of list and hidden-field presets ("switching type drops the unrendered preset props"). The rendered prefilled values stay in the draft. A person who looks at another type and comes back sends half a preset: the visible given props without the list or hidden ones, and without `expect_generation`. If a guard reads the list prop (`''x'' in message.tags`), the post takes another entry. Either refuse list and hidden-field presets on a compose button in this ticket (dataentryconfig cannot be checked from realtimecfg, so hidden would need a runtime guard), or keep the preset with the draft until reset or the next prefill, independent of `formDecision`.'
severity: significant
resolution: ADDRESSED The preset lives only in formDecision.compose and the visible given values are not in the draft, so a type switch ends compose mode whole and no half preset can be posted (AC 8 ComposeModeTypeSwitch, AC 10).
status: addressed
---
