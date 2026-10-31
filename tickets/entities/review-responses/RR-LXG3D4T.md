---
id: RR-LXG3D4T
type: review-response
title: Conflict window for shown values is undefined
finding: Define the value shown by a conflict as the value the conflict refetch returned for that field or slot, shown from the report on; a time window is either too loose or gives false counterexamples.
severity: significant
resolution: A conflict report adds to the tab's shown values exactly the fields of the refetch its current save received; the record is reset when each save starts.
status: addressed
---
