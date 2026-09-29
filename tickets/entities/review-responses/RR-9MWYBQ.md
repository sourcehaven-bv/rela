---
id: RR-9MWYBQ
type: review-response
title: Custom type validations not checked at load
finding: 'A literal inside values but failing a custom type''s validations: regex still fails on write; the doc sentence could suggest full validation.'
severity: minor
resolution: 'Doc sentence narrowed to string literals and enum values and states that other checks such as validations: still apply on write. Extending to validations stays out of scope.'
status: addressed
---
