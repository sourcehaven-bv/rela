---
finding: TestEngine_CreateEntity_WithTemplate tests {{new.kind}} when kind exists. Missing property likely interpolates to empty string but this should be tested and documented.
id: RR-lier
resolution: Added TestEngine_CreateEntity_TemplateMissingProperty which documents that missing properties interpolate to empty string, resulting in default template being used.
severity: minor
status: addressed
title: No test for template interpolation with missing property
type: review-response
---
