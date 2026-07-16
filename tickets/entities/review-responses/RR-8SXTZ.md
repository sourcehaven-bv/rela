---
id: RR-8SXTZ
type: review-response
title: Duplicate step titles cause Vue key collisions in the stepper
finding: The wizard stepper keyed on step.title; validateForms only requires a non-empty title, not uniqueness, so two steps titled the same collide on key.
severity: minor
resolution: Keyed the stepper v-for on the visible-step index (sIdx) instead of title. visibleSteps recomputes as a unit, so index is stable enough here.
status: addressed
---
