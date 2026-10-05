---
id: RR-2FGY1O
type: review-response
title: Reader 404 check did not test the grant or uniformity
finding: '[security] Bare POL-2 is excluded by the world anyway; POL-2@draft was never requested and a hidden face was not compared with a missing id.'
severity: minor
resolution: The API test now requests POL-2@draft and POL-999 and expects the same null (404) as for the hidden faces. Body equality stays with the Go handler tests per AGENTS.md.
status: addressed
---

[security] Bare POL-2 is excluded by the world anyway; POL-2@draft was never
requested and a hidden face was not compared with a missing id.
