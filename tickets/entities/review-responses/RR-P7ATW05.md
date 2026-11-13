---
id: RR-P7ATW05
type: review-response
title: ExtractHighestNumber keeps a second matching rule
finding: ExtractHighestNumber still matched through ParseEntityID, a different rule from SequentialNumber.
severity: minor
resolution: ExtractHighestNumber now loops over SequentialNumber, so one rule remains.
status: addressed
---
