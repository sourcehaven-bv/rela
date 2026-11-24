---
id: RR-F4UREG8
type: review-response
title: Edge cases in body-comment detection are unspecified
severity: significant
resolution: 'Spec added: closures count as body; generated files (Code generated) skipped; cgo and /// reference allowed; JSDoc tag-only blocks allowed; Vue script parsed via eslint-plugin-vue; template comments out of scope.'
status: addressed
---

Closures, comments in composite literals and switch cases, generated files, cgo
preambles, /// reference, JSDoc type tags and Vue script blocks are not covered.
