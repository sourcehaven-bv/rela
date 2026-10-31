---
id: RR-C7SO6H2
type: review-response
title: Newer read can land after a refusal
finding: '[security] A refusal sets landed only to the refused read''s sequence, so a newer read waiting in prepare can put the entity back on screen after onRefused cleared it (pre-existing).'
severity: minor
resolution: 'A refusal sets landed = issued. Unit test: a refusal voids a newer read waiting in prepare.'
status: addressed
---
