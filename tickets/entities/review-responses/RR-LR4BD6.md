---
id: RR-LR4BD6
type: review-response
title: tagPermissionGuard ignores the request on ctx
finding: It re-walks membership via ForPrincipal and checks by id only.
severity: minor
resolution: 'Reuses the acl request on ctx; TestTagPermissionGuard. Per entity only: the permission API has no face parameter.'
status: addressed
---
