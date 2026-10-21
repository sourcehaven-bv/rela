---
id: RR-RDCAU0
type: review-response
title: No upgrade note for deployments that already have an acl.yaml
finding: Under an existing acl.yaml, sync and the settings saves go from open to refused until an operator grants the new permissions, and the docs did not say so. (cranky-code-reviewer)
severity: minor
resolution: Both sections of the ACL guide end with an Upgrading note telling operators to grant git:sync and settings:edit to the roles that used the features.
status: addressed
---
