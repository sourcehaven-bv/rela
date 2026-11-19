---
id: RR-MKFYVN1
type: review-response
title: No test for a role-less guarded branch
finding: A branch with no roles admits anyone; Blocked then holds for any caller.
severity: minor
reason: The admit rule is unchanged and already tested; Blocked only reads it. The fork and role-not-held cases pin both outcomes.
status: wont-fix
---
