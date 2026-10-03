---
id: RR-K14EIG
type: review-response
title: buildACL doc comment captured by ValidateACLPolicy
finding: Inserting ValidateACLPolicy split the buildACL doc block so the buildACL godoc attached to the wrong function.
severity: significant
resolution: Moved the doc block back above func buildACL; validateResolvedPolicy now runs directly after the metamodel load in prepare with its own comment.
status: addressed
---
