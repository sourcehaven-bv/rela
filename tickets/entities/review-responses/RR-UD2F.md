---
id: RR-UD2F
type: review-response
title: SelectWidget display stringValue is a latent footgun for arrays
finding: |
  useStringValue([...]) returns String([...]) which is "a,b". If anything routes an array to SelectWidget in display mode (config drift, a future widget:select override on a list-typed field, a CustomView whose propType is empty but values is multi-valued), one Badge renders with literal text "a,b" and Badge's lookup uses "a,b" as the value key. Silently masked today only because synthDef always sets list:true for propType-having fields.
severity: significant
status: addressed
resolution: |
  Added a safeStringValue computed to SelectWidget that defensively coerces array input. Single-element arrays render the unwrapped value (preserves existing PropertyDisplay path which receives [val] arrays today). Multi-element arrays render the first element with a console.warn("received multi-element array in display mode; rendering first only -- consider widget: multi-select"). Behaviour is identical today (no array of length > 1 reaches SelectWidget under current routing); the guard catches TKT-HOIX1's widget-override surface and any future config drift. Tests pin both the single-element no-warn path and the multi-element warn-and-render-first path.
---
