---
id: RR-CTX6
type: review-response
title: Ceiling guard blind to the relation-grant policy read
severity: significant
status: addressed
finding: ceilingguard_test.go matches policy.Roles[ only, so it cannot see the relation-grant path's read
  of policy.RelationWriteGrants. The read is safe today because it yields a permission name and the check
  goes through grantsPermission, but nothing pinned that property.
resolution: Added TestRelationGrant_PermissionCheckRoutesThroughGrantsPermission asserting that a ceiling
  withholding the permission still denies; mutation-verified it fails against a direct map read.
---
