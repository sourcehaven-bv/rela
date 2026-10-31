---
id: RR-CB1W1X
type: review-response
title: The composer's session hint does not depend on field values, and editable given props can change the route
finding: 'The plan''s edge case says that when the person changes a prefilled enum, "the composer''s session hint says where". It does not. `sessionHint` is computed per message type from `transitions[].toLabels` and ignores the values (frontend/packages/rela-components/src/components/channel/RlChannelComposer.vue:230-238). For a branching type it says "may move to Approved or Drafting". In the guide''s branching example, which AC 13 changes to `compose: true` (docs-project/entities/guides/GUIDE-realtime.md:942-949), Revise opens the composer with `decision: revise` editable. The person can switch it to approve and send. That moves to the final Approved state and runs its outputs without the confirm the Approve button would ask. The load''s hidden-set reasoning also holds only while the given props are unchanged. Fix: carry the given props as unrendered, as a list preset is carried, or show them read-only. Approach 8''s argument against locking (the server cannot enforce it) is about security, but this is UX: the label promises the given values. At least remove the false claim and add an AC for an edited routing prop.'
severity: significant
resolution: 'ADDRESSED Given props are locked: not rendered as fields, shown read-only in a strip, and added by the app at Send. The session hint in compose mode names the button''s target. The false edge-case claim is removed (AC 8, Approach 8, Alternatives).'
status: addressed
---
