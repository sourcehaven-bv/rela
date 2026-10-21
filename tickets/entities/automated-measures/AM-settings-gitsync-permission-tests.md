---
id: AM-settings-gitsync-permission-tests
type: automated-measure
title: Settings-page saves and git sync answer 403 without their permission
description: Table tests fail when PUT /_settings, PUT /_palette, the logo upload or delete, the theme import or POST /_git/sync runs for a principal without settings:edit or git:sync under a policy, or under --read-only. A refused call must persist nothing. The decl-header goldens pin the 403 for every non-admin identity of the golden fixture.
kind: test
location: internal/dataentry/settings_authz_test.go; internal/dataentry/handlers_git_test.go; internal/access/mode_test.go (TestMode_Decisions); internal/dataentry/testdata/golden/decl-header/{settings,palette,theme-*}
status: active
---
