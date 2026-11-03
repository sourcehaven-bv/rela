---
id: RR-RN50Q9
type: review-response
title: Failed anchor re-read after a handoff keeps stale holders and a stale waiting_on_me
finding: 'internal/realtime/waiting.go:144-155 rereadAnchor returns early on any error and leaves c.anchor and c.ops untouched. The post answer then shows the old holders and the cached heldRole(lead)=true from the post gate, so the poster who just handed the role away is told waiting_on_me: true. An agent following the documented loop would act on that. Fix: on a failed re-read, clear c.ops.roles and drop WaitingOnMe (or omit waiting_for) instead of rendering the pre-write view. Also build the address with the anchor''s Ref directly instead of a FormatStateRef -> ParseAddress round trip.'
severity: minor
resolution: A failed re-read clears the anchor props so the answer shows no holders and waiting_on_me false; tested by the failed re-read subtest
status: addressed
---
