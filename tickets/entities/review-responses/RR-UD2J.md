---
id: RR-UD2J
type: review-response
title: DateWidget silent fallback is deliberate but undocumented
finding: |
  formatDate(stringValue.value) ?? stringValue.value -- no console.warn. The plan apparently mentioned warning. Silent fallback is the right call (warnings in render functions spam the console on every reactive tick), but the design-review trail doesn't say so. Future-you will second-guess the missing warn.
severity: minor
status: addressed
resolution: |
  Added a comment in DateWidget.vue explicitly documenting the deliberate silent fallback: "this computed runs on every reactive tick, so warning here would spam the console for any stale/in-progress date value. The raw-string passthrough is the right visible signal that something is off." Anyone touching the widget now knows why there's no warn, and won't reintroduce one as "a missing best practice."
---
