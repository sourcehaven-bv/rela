---
id: RR-NBX0RY
type: review-response
title: Rotated token lost if ctx cancelled before Put
finding: Put on a cancelled ctx loses the new refresh token.
severity: significant
resolution: 'Plan R1: Put on WithoutCancel with own timeout and one retry; cancellation test.'
status: addressed
---
