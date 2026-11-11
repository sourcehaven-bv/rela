---
id: RR-1D9LL9K
type: review-response
title: Elevated scope has no face dimension
finding: elevated_updates keyed by type only; a faced type's faces were all in scope.
severity: minor
resolution: A faced type must be keyed type@face at config load; a bare key is refused for a faced type and a face is refused for a faceless one. ScopedUpdates matches type@face for faced rows. Tests in dataentryconfig and TestScopedUpdates_Faces.
status: addressed
---
