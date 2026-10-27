---
id: RR-XDJGTI
type: review-response
title: Source face selection must not reach the related query
finding: Copying AllFaces/AtFaces from the source read multiplies counts or names faces the related type lacks.
severity: significant
resolution: The related query always uses InWorld of the request's world (default world when none); store.ValidateRollupQuery refuses any non-world selection.
status: addressed
---
