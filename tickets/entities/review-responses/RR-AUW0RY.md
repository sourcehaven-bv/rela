---
id: RR-AUW0RY
type: review-response
title: Resolve checks git:sync after parsing the body
finding: handleV1ConflictResolve decoded JSON and checked the path before the git:sync gate, so a refused caller could get a 400 instead of a 403.
severity: minor
resolution: The gate moved to the top of handleV1ConflictResolve, before the body is read.
status: addressed
---
