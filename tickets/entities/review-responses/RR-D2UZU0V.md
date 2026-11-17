---
id: RR-D2UZU0V
type: review-response
title: Member walk runs before a Request exists
finding: walkMembers runs in ResolveDirect before ForIdentity, so a Request-bound instant cannot reach the membership condition.
severity: significant
resolution: 'Plan: capture the instant at the request edge, carry it on Identity, use it for the walk and the Request.'
status: addressed
---
