---
id: RR-9L87EIA
type: review-response
title: recheckEnds does not recheck the tail face of an existing source
finding: A content-scoped edge from a face deleted before the Tx would still commit.
severity: minor
resolution: 'endType refuses an end whose named face is no longer in the family, as a missing end. No dedicated test: the engine fixture has no faced type.'
status: addressed
---
