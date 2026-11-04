---
id: RR-4ZHC86U
type: review-response
title: Announced-once test cannot see a second announcement
finding: "The test asserted the same live-region text after the second append, which a repeat announcement would also produce."
severity: minor
resolution: "The second batch now holds a shown message too; the test asserts the live region changed and holds no notice text."
status: addressed
---
