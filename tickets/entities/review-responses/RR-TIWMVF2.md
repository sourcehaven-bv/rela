---
id: RR-TIWMVF2
type: review-response
title: Scales without the base scale skip the ratio check
finding: -scales 5 or 1,1 were accepted.
severity: minor
resolution: 'Addressed: parseScales requires the config''s base scale and rejects duplicates; TestParseScales.'
status: addressed
---
