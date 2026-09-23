---
id: RR-ASBQ8T
type: review-response
title: searchBodies read every state of each candidate id
finding: The body fetch was WHERE id = ANY(ids) with no face, correct only because a map lookup discarded the extra rows; it also transferred bodies of faces the world or ACL withheld.
severity: significant
resolution: The fetch joins on the exact (id, face) pairs the gated query resolved, via unnest of two arrays, so it reads no row the gate did not admit.
status: addressed
---
