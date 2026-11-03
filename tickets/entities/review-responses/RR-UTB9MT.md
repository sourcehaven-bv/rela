---
id: RR-UTB9MT
type: review-response
title: Variant slug lists not checked against endpoints
finding: Renaming a slug silently drops its nop/readonly/JWT-refusal cases.
severity: minor
resolution: A slug in goldenSwitchSlugs or goldenJWTRefusalSlugs that matches no endpoint now fails the test.
status: addressed
---
