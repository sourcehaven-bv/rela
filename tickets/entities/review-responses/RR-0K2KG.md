---
id: RR-0K2KG
type: review-response
title: NormalizeHeaders does not belong on Workspace
finding: 'NormalizeHeaders(content string) string is a pure function with no state dependency. Workspace is a stateful domain session. Bolting a stateless string transformation onto it just to satisfy arch lint is wrong abstraction level. Better alternatives: move to model (entity content is a model concept), create internal/content package, or put on Entity as a method.'
severity: significant
resolution: 'Plan updated: NormalizeHeaders will move to model package as a standalone function model.NormalizeContentHeaders(content string) string. CLI calls model.NormalizeContentHeaders() directly — model is already a common dependency.'
status: addressed
---
