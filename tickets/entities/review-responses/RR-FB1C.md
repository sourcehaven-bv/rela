---
id: RR-FB1C
type: review-response
title: 'C3: AC 5 references a header Badge that does not exist'
finding: |
  PLAN AC 5 asserts the "entity-header Badge rendering same property elsewhere reflects the new value". EntityDetail's header has type-badge, title, action buttons — NO property Badge. The only Badge usage in EntityDetail is inside table cells. So the AC is describing a behaviour with no render target.
severity: critical
status: addressed
resolution: |
  Rewrote AC 5 to a real surface: "When a property X is edited and the server confirms, *the next loadView() refresh of EntityDetail* shows the new value across all consumers (PropertyDisplay/SelectWidget badges in other sections, content section's interpolation, etc.). The intermediate window — between commit and `onPropertyApplied` firing — only affects the local `SectionEditForm`'s formData; no other section reads from formData."

  This kills the "two sources of truth" framing entirely and resolves S11. Test becomes: after `applyServerProperty` fires, the host's `entry.properties[p]` is updated; on next render any sibling section reading `entry.properties[p]` sees the new value. Standard Vue reactivity, no special tracking.
---
