---
id: RR-YONYXR
type: review-response
title: Client ceilings pass git:sync on to delegated clients
finding: permissionCeiling.permits never delegates config:edit but has no such rule for git:sync, so a client with an unnarrowed ceiling inherits it. Neither code nor docs said whether that is intended. (cranky-code-reviewer minor; rela-security-reviewer nit)
severity: minor
resolution: The git:sync section of the ACL guide now states that a client ceiling passes git:sync on unless it narrows permissions, and that only config:edit is never delegated. The behaviour is unchanged.
status: addressed
---
