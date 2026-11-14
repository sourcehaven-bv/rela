---
id: RR-Q0KY335
type: review-response
title: rela flow validate reports no flows next to valid headless flows
finding: The line said the project declares no flows when there is no flows.yaml, followed by a count of valid headless flows.
severity: minor
resolution: The line now says only that no flows.yaml was found; the headless line no longer names schema.yaml.
status: addressed
---
