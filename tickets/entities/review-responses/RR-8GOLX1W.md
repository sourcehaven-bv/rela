---
id: RR-8GOLX1W
type: review-response
title: "Equality test does not prove the prime was taken"
finding: "TestNavStatus_PrimedCountHonorsLocalRoleVisibility passes with or without the fix, and its comment implied a primed-vs-unprimed comparison."
severity: minor
resolution: "Reworded the comment, and the test now also asserts no GetRelation read happened. Without the fix it fails on that assertion; the count matches either way."
status: addressed
---
