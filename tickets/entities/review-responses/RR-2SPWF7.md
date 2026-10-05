---
id: RR-2SPWF7
type: review-response
title: 'Design: family delete is unreachable through the HTTP API'
finding: BUG-1YN750 is a bare-id family delete. DELETE /api/v1/policies/POL-1 answers 404 on a faced type and every web-app delete is face-addressed, so a spec that drives the bare HTTP delete would pass today and prove nothing once flipped on.
severity: significant
resolution: 'The fixture declares a detail-page Lua action (available_on policy) that calls rela.delete_entity on the bare id: the one web-app path that reaches a family delete. The fixme spec clicks it as the draft-only editor and asserts the published face survives.'
status: addressed
---

BUG-1YN750 is a bare-id family delete. DELETE /api/v1/policies/POL-1 answers 404
on a faced type and every web-app delete is face-addressed, so a spec that
drives the bare HTTP delete would pass today and prove nothing once flipped on.
