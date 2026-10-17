---
id: RR-YMQ2B9
type: review-response
title: Face-scoped grants are invisible to the map, so --expect can pass while the runtime allows more
finding: grantingAttributions credits writes type-granularly; a `type@face` grant was not reported.
severity: significant
resolution: 'TypeAccess gains Faces: per declared face, the verbs a global role grants on that face only (Request.RoleGrantsVerbOnFace, ceiling applied). Expectation keys accept `type@face`. Face grants through entity-local roles remain unlisted; documented.'
status: addressed
---
