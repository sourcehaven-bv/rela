---
id: entity-detail-list-unmount-test
type: automated-measure
title: 'Test: navigating away from a populated display:list entity-detail section does not crash Vue on unmount'
description: 'Regression test for BUG-UAIR8C / issue #997. The inline e2e fixture configures a `feature` view with a populated `display: list` relation section (FEAT-001 blocks FEAT-003) so a per-row SectionEditForm + AutoSaveIndicator mounts. The test installs an `app.config.errorHandler` shim on the live Vue app, asserts the list-section row form is mounted, then navigates away via an in-SPA router link and asserts the errorHandler never fired. Fails with `Cannot destructure property ''bum'' of ''e'' as it is null` pre-fix; passes post-fix. Establishes the pattern of hooking `config.errorHandler` (not `pageerror`/`console.error`) to catch framework-swallowed unmount errors.'
kind: test
location: e2e/tests/entity-detail-list-unmount.spec.ts (+ feature `views` block in e2e/tests/fixtures.ts; `expectListSectionRowMounted`/`navigateAwayViaRouter` in e2e/pages/entity.page.ts)
status: active
---
