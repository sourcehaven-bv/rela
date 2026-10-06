---
id: AM-rename-authorizes-every-face
type: automated-measure
title: Rename is denied when any face it re-keys is denied
description: A principal lacking the grant for one face cannot rename the family; a grant on every face renames all of them.
kind: test
location: internal/entitymanager/familyrename_acl_test.go (TestFamilyRename_DeniedUnlessEveryFaceIsRenamable, TestFamilyRename_EveryFaceCapturedAndAudited, TestFamilyRename_FaceAddedDuringRenameIsAuthorized, TestFamilyRename_DryRun)
status: active
---
