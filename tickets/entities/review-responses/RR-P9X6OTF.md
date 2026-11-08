---
id: RR-P9X6OTF
type: review-response
title: Wildcards outside internal/acl still cover the run type
finding: comments on ["*"] made runs commentable; aclaudit matches "*" for attestation.
severity: minor
resolution: CommentPolicy.Commentable excludes managed types under the wildcard (TestWithFlowRuns_CommentWildcardSkipsRuns). aclaudit only reports; it grants nothing.
status: addressed
---
