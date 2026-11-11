---
id: RR-IS9YCVG
type: review-response
title: CheckEmbeddedSPA returned nil for every headless caller
finding: A future UI-serving caller would pass silently under the tag.
severity: minor
resolution: CheckEmbeddedSPA stays strict; rela-server skips it when Headless and logs the variant. The headless test asserts it errors.
status: addressed
---
