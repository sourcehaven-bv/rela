---
id: RR-P3K8UG
type: review-response
title: Re-baselining at mount destroyed the guard's reference point
finding: 'After load settled, MilkdownEditor did `loadedValue = serialize()`, replacing the user''s original bytes with the post-round-trip form. guardWriteBack then compared the churned output against itself and always reported ''unchanged'', so churn-suppressed was unreachable for any body that churns at load. Verified: mounting ''Title\n=====\n\nsome text\n'' returned verdict ''unchanged'' with value ''# Title\n\nsome text\n'' — the setext heading already converted.'
severity: critical
resolution: 'Split the single `loadedValue` into `originalValue` (pristine, only ever assigned from the prop or the reload watcher) and `settledValue` (re-derived, used solely to suppress the listener echoing our own load). The guard takes originalValue. Verified by reintroducing the re-baseline: four component tests fail, then pass on restore.'
status: addressed
---

## Finding

`MilkdownEditor.vue:545` did `loadedValue = serialize()` once loading settled,
with the comment "so the guard compares against what the editor actually holds
rather than the bytes it was handed". That is the bug written down as if it were
the fix: the bytes it was handed are exactly what the guard needs.

Measured directly, mounting `Title\n=====\n\nsome text\n`:

```text
verdict: unchanged
value:   # Title\n\nsome text\n
```

The guard reports nothing happened while handing back different bytes.
`churn-suppressed` was unreachable for the 44% of the corpus that churns.

## Interaction with the other critical

This compounded RR-6OIIOE. The guard was not called, and had it been, it would
have been comparing the wrong baseline. The corpus test could not catch either,
because it exercises remark in isolation and never mounts the component.

## Resolution

Two variables with distinct jobs:

- `originalValue` — the exact bytes the parent handed over. Assigned only from
the initial prop and the reload watcher. This is what the guard compares
against.
- `settledValue` — re-derived after load, used only to suppress the listener
echoing our own serialization back at the parent.

Pinned by component-level tests over setext headings, `*` bullets, `***` rules
and loose table padding, each asserting both that nothing is emitted and that
`guardedValue()` returns the original bytes with verdict `churn-suppressed`.
Reintroducing `originalValue = settledValue` fails all four.
