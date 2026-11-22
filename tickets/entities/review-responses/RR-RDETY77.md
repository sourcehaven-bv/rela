---
id: RR-RDETY77
type: review-response
title: Startable reads the subject once per manual flow
finding: openPermissions reads the raw subject for each manual flow on the type, so a type with several manual flows reads it several times per page load.
severity: minor
reason: A type has a handful of flows at most and a start condition is optional; the read is by id. Passing the gated entity instead would evaluate guards on a redacted read, unlike every other flow guard.
status: wont-fix
---
