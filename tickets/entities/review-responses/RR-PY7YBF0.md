---
id: RR-PY7YBF0
type: review-response
title: Missing elevated update tests
finding: No test for a hidden id with an out-of-scope property; the guard test did not show the guard asks about the caller; no HTTP case for a hidden row of an in-scope type.
severity: minor
resolution: Added the hidden id plus out-of-scope case (ErrEntityNotFound); the guard test allows bob only and runs as bob and alice; added the HTTP case with secret in scope.
status: addressed
---
