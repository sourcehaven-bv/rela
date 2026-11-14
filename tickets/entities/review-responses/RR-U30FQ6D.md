---
id: RR-U30FQ6D
type: review-response
title: expect_denied could match a stale denial
finding: A denial recorded earlier could satisfy a later expect_denied whose call failed otherwise.
severity: minor
resolution: expect_denied checks that the raised error ends with the last recorded error.
status: addressed
---
