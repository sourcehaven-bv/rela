---
id: RR-0IOT2V8
type: review-response
title: No budget test for the run store read
finding: The counting store wraps the entity store only; RunsByID is not counted.
severity: minor
reason: The page reads the run store once through RunsByID with the page ids, outside any loop over rows; flowrun.Store has no counting harness and collection-reads.md covers the entity read paths.
status: wont-fix
---
