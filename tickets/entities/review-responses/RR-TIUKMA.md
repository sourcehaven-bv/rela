---
id: RR-TIUKMA
type: review-response
title: E2E fixture's entry properties section silently lost inline edit with no comment
finding: 'The migration added `render: input` to the fixture''s `Implements` list section (with a comment explaining why) but not to the `Task` properties section immediately above it in the same YAML block — fields `status` and `assignee`. No current spec asserted inline edit there, so nothing failed. That is the hazard: the fixture silently acquired a display-only entry section, and a future test written against it would find a PropertyDisplay where its author expected an editable form.'
severity: significant
resolution: 'Left at the display default deliberately — it gives the suite a genuinely mixed page, which the new view-section-render-mode.spec.ts now depends on — but added an explicit comment saying so and telling a future spec author to add `render: input` rather than assume the pre-TKT-HOIX1 default. The section is now covered by three assertions rather than being untested.'
status: addressed
---

Turning the omission into a deliberate, documented, and *tested* choice was
better than migrating it: the suite previously had no page exercising both
render modes at once, and this section supplies exactly that shape for
RR-5KFD7W's new specs.

Note: an earlier version of this comment used backticks around `render: input`,
which terminated the surrounding TypeScript template literal and broke the
fixture's parse. Caught by Playwright's transform step. The comment now avoids
backticks inside `DATA_ENTRY_YAML`.
