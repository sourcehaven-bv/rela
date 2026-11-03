---
id: RR-SIXDMD
type: review-response
title: OpenAPI description line exceeds 120 characters
finding: 'internal/openapi/channels.go:484 is 127 characters after the edit, over the repo''s 120-character line length. Fix: split the string concatenation before "outside them".'
severity: nit
resolution: Split the string before "outside them"; no line of the edit exceeds 120 characters.
status: addressed
---
