---
id: RR-1OXIDN
type: review-response
title: typeText collapsed whitespace inside struct tags
finding: A literal with a tag containing double spaces got a mismatched wrapper type and failed to compile.
severity: nit
resolution: Literals whose type has a struct tag are left uninstrumented; tested; documented in Limits.
status: addressed
---
