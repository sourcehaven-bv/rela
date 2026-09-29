---
id: AM-family-delete-every-face
type: automated-measure
title: Family delete authorizes, captures and audits every face
description: A bare-id DeleteEntity on a faced entity is denied unless the principal holds delete on every face it removes, deletes nothing when denied, and records one version capture and one audit record per removed face. Runs over memstore, fsstore, and the sqlite and postgres backends under their build tags.
kind: test
location: internal/entitymanager/familydelete_acl_test.go (TestFamilyDelete_DeniedUnlessEveryFaceIsDeletable, TestFamilyDelete_EveryFaceCapturedAndAudited)
status: active
---
