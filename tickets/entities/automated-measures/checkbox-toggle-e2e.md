---
id: checkbox-toggle-e2e
type: automated-measure
title: Checkbox toggle e2e coverage (pending)
description: Regression harness for bug BUG-9RANL; currently test.skip in e2e/tests/checkboxes.spec.ts pending a clickable-checkbox harness.
kind: test
location: e2e/tests/checkboxes.spec.ts
status: proposed
---

Preventive measure for BUG-9RANL: once the harness can drive disabled GFM
checkboxes reliably, unskip the `clicking a checkbox persists the toggle on the
server` test in e2e/tests/checkboxes.spec.ts. Until then, the test stub is
preserved so the unskip is a mechanical change.
