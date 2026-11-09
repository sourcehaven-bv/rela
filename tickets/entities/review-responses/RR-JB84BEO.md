---
id: RR-JB84BEO
type: review-response
title: Schema loaded more than once per pull
finding: The mapping was compiled against one schema load and the run used another.
severity: minor
resolution: pull recompiles the mapping against the schema in the run's deps, so one pull reads one schema.
status: addressed
---
