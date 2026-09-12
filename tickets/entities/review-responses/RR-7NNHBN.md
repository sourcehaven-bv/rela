---
id: RR-7NNHBN
type: review-response
title: No test exercised the component's real save path
finding: Every write-back guard test called guardedValue() directly on the mounted component, reaching past the wiring the application actually uses. The only emit assertions were negative, against a listener that never fires under happy-dom for a programmatic dispatch — so they could not distinguish 'correctly suppressed' from 'never ran'. 179 tests passed while the headline invariant was broken in two independent ways.
severity: significant
resolution: Added component-level tests over four churning body shapes asserting both no-emit and original-bytes-returned; two tests that drive the registered markdownUpdated callback directly to exercise the real emit path; a rebaseline-on-reload test; and pure unit tests for the extracted decideEmit(). All confirmed non-vacuous by reintroducing each bug.
status: addressed
---

## Finding

The suite tested the guard function and the component separately, and nothing
tested them connected. That is precisely the seam where both criticals lived.

The reviewer's summary of the lesson is worth keeping: *a test that reaches past
your own wiring to poke the unit is testing that the unit compiles, not that the
feature works.*

## Resolution

Four kinds of test added:

1. **Component-level, churning bodies** — setext heading, `*` bullets, `***`
rule, loose table padding. Each asserts no emit AND that the guard returns the
original bytes with verdict `churn-suppressed`.
2. **Listener-driven** — invokes the registered `markdownUpdated` callback
directly, since Milkdown's debounced listener does not fire for a programmatic
dispatch under happy-dom. This is the only way to assert the emit path
positively.
3. **Reload rebaselining** — a new value from the parent must move the
baseline, or the guard measures against a body the editor no longer holds.
4. **Pure `decideEmit` unit tests** — the decision logic without an editor.

Each was verified by reintroducing the corresponding bug and watching the right
tests fail: re-baselining fails 4, bypassing the guard fails 1.
