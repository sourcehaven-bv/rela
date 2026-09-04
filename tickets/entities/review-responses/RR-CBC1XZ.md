---
id: RR-CBC1XZ
type: review-response
title: '.display-checkbox:disabled loses the specificity race — display arm rendered not-allowed, inverting the stated intent'
finding: 'The new `.display-checkbox:disabled { cursor: default }` rule never applied. `input[type=''checkbox'']:disabled` is (0,3,1); `.display-checkbox:disabled` is (0,3,0). Vue''s scoped compilation appends `[data-v-x]` to both, shifting them equally, so the element rule wins outright regardless of source order. Every read-only boolean therefore rendered `cursor: not-allowed` — the exact behaviour the comment above the rule said it was preventing, and a regression against the pre-change code where `cursor: default` was the only rule and did apply. The ticket''s Non-goals opens with "No behaviour change"; hover affordance is user-visible behaviour.'
severity: critical
resolution: 'Rewrote the general rule as `input[type=''checkbox'']:disabled:not(.display-checkbox)`. `:not()` takes its argument''s specificity, making it (0,4,1) AND — more importantly — making it structurally unable to match the display arm, so a later edit cannot silently re-lose the race. Added `resolves the read-only cursor for the display arm` to widgets.test.ts, which injects the rules with the real `[data-v-x]` suffix and asserts computed cursor per arm. Verified in a real browser against the compiled stylesheet: display arm `default`, ACL-denied edit arm `not-allowed`, live edit arm `face`.'
status: addressed
---

Confirmed independently before fixing, rather than taken on the reviewer's word
— jsdom resolves the cascade correctly, so the bug reproduces in a few lines:

```
display arm cursor: not-allowed     <- should have been "default"
edit arm cursor   : not-allowed
```

The instructive part is that both tests I had written were green while this was
live. They asserted that a class was *present*, which cannot see whether the
rule hanging off that class *applies*. The new test fails with
`expected 'not-allowed' to be 'default'` when the `:not()` guard is removed, so
it reproduces the defect rather than merely covering the line.
