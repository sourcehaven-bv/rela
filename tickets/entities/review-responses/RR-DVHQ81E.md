---
id: RR-DVHQ81E
type: review-response
title: Strict decoding would reject keys ignored today
finding: metamodel decode ignores unknown keys; flowcfg uses KnownFields, so a typo would fail the load after migrating.
severity: minor
resolution: 'Plan: the migration decodes each automation strictly first and refuses naming the key.'
status: addressed
---
