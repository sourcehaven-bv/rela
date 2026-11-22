---
id: AM-table-column-sizing-phone-laptop
type: automated-measure
title: Table column sizing on a phone and a laptop
description: Unit tests pin that a five-column list fits a 1024px page beside the sidebar; e2e tests pin a readable phone title with kept cells under it, the full table with headers at 1024px, and the phone Overview title in the menu bar.
kind: test
location: frontend/packages/rela-components/src/components/table/columnSizing.test.ts, e2e/tests/mobile-layout.spec.ts, e2e/tests/list.spec.ts
status: active
---

Fails when a phone list squeezes its title, a laptop list drops to the compact
layout, or the phone Overview puts its heading below the notice.
