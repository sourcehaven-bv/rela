---
id: RR-24V4VNO
type: review-response
title: Hidden or missing peer refused only after the entity write
finding: Phase A only warned target_not_found for an unreadable or missing peer; the write phase then refused it with 422 after Phase B had patched the entity or the POST had created it. A hidden peer of a disallowed type therefore still mutated the entity.
severity: significant
resolution: checkRelationEdges now returns danglingPeerError for a peer with no readable type before any write; the dead warning branch is gone. TestRelationWrite_HiddenDisallowedPeerAnswersAsMissing pins it (fails with the old skip).
status: addressed
---
