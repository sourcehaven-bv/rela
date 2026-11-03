---
id: RR-0EHQECF
type: review-response
title: "Budget pins include ACL set-up reads"
finding: "nav_status_budget_test.go: the policy and resolver were installed inside the measured op, so an ACL construction change would move the pins."
severity: minor
resolution: "The tests use their own fieldPolicyReads loop, which installs the policy and resets the counter before the op."
status: addressed
---
