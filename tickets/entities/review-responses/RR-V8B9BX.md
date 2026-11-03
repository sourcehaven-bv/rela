---
id: RR-V8B9BX
type: review-response
title: mayGitSync wrapper duplicates permitsGitSync
finding: mayGitSync only forwarded a.acl to permitsGitSync, giving one rule two names, and the archplan reasons cited the wrapper.
severity: nit
resolution: Removed mayGitSync; all callers use permitsGitSync and the reasons name it.
status: addressed
---
