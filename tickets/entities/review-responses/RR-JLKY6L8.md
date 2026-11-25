---
id: RR-JLKY6L8
type: review-response
title: Baseline must not key on line numbers
finding: Line-number keys churn on every edit above the comment and force baseline rewrites.
severity: significant
resolution: Key the baseline on file plus enclosing function name plus comment count. Fails when a count rises or a key is new.
status: addressed
---

Line-number keys churn on every edit above the comment and force baseline
rewrites.
