---
id: RR-BS0O4
type: review-response
title: Test plan over-asserts on URL encoding
finding: 'AC2 in the plan asserts the exact percent-encoded form of return_to. document-links-roundtrip.spec.ts:99-102 already established the better pattern: parse via new URL(...) and assert on searchParams.get(''return_to'') decoded. Adopt that to avoid future flakes when vue-router changes encoding rules.'
severity: nit
resolution: 'Plan updated: AC2 explicitly uses new URL(...) + searchParams.get(''return_to'') and asserts on the decoded value, matching document-links-roundtrip.spec.ts:99-102.'
status: addressed
---
