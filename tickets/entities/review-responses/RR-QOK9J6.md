---
id: RR-QOK9J6
type: review-response
title: rolesHidingField counts roles that do not read the type
finding: 'rolesHidingField reported roles with a visible: block on the type even when they do not read it, so they hide nothing.'
severity: nit
resolution: rolesHidingField skips roles for which acl.ReadFaces reports no read on the type. TestListVisibilityConflicts has a bystander role with a block and no read, and it is not reported.
status: addressed
---
