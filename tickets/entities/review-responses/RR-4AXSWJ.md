---
id: RR-4AXSWJ
type: review-response
title: has_more and next_after do not match the page direction
finding: For the newest page next_after is the newest seq while has_more means older messages exist; a before page has no cursor.
severity: minor
resolution: Newest and before pages emit next_before; after pages keep next_after; has_more is documented as the page being full. Wire type and OpenAPI updated.
status: addressed
---
