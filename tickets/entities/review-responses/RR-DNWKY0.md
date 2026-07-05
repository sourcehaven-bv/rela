---
id: RR-DNWKY0
type: review-response
title: data-visible attribute is production DOM surface existing only for tests
finding: data-visible attribute on the indicator exists only for test assertions; the autosave-hidden class already carries the same signal, so it's production DOM surface that exists only for tests.
severity: nit
resolution: 'Removed the data-visible attribute from AutoSaveIndicator''s template; tests now assert on the autosave-hidden class instead. Verified gone in the real app (hasDataVisible: false).'
status: addressed
---

**Finding:** `data-visible` on the indicator exists only so tests can assert on
it; the `autosave-hidden` class already carries the same signal.

**Fix:** Drop `data-visible`; assert on the `autosave-hidden` class instead.
