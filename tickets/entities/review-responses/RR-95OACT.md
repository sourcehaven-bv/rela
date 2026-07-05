---
id: RR-95OACT
type: review-response
title: 'Test gaps: empty-heading case, EntityDetail heading suppression, cards/list slot override'
finding: 'No test for heading:'''' (the falsy-but-present value the wiring produces, which triggers the critical bug), and no test that the cards/list #indicator slot override still wins over the changed default fallback.'
severity: minor
resolution: 'Added SectionEditForm tests: heading:'''' → headless path (no header row, single indicator), and a #indicator slot-override test (host slot wins, default AutoSaveIndicator absent). EntityDetail-level heading-suppression test deferred (no EntityDetail unit harness exists; the guard is unit-covered and the seam was manually verified in the real app).'
status: addressed
---

**Finding:** No test for `heading: ''` (the exact falsy-but-present value the
wiring produces — the critical bug's trigger). No test asserting the cards/list
`#indicator` slot override still wins over the changed default fallback.

**Fix:** Add `mountForm({ heading: '' })` test (headless path, header row
absent), and a `#indicator` slot-override test (host slot wins, default absent).
EntityDetail-level heading-suppression test deferred — component covers the seam
and no EntityDetail unit harness exists yet.
