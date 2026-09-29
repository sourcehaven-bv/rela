---
id: RR-GL2LDN
type: review-response
title: No conformance case for a copy onto an existing row
finding: Publishing over an existing row lands as an UPDATE with origin; untested on either backend.
severity: minor
resolution: Added SweepOrigin/CopyOntoAnExistingRowIsAnUpdate; passes on sqlite and postgres.
status: addressed
---
