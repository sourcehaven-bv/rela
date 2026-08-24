---
id: RR-CBM1TS
type: review-response
title: 'The two added tests are change-detectors for a class attribute, and their comment oversold them as pinning the read-only treatment'
finding: 'The tests assert only that `.display-checkbox` is present on the element. They cannot see whether the rule that class hangs off actually wins the cascade — which is exactly the defect in RR-CBC1XZ, in the same file, with 60 green tests over it. The comment claimed the class was "load-bearing … pinned by tests", which a future reader would reasonably read as a guarantee that does not exist. Separately, the first test looped `for (const modelValue of [true, false])` with no subtest boundary, so a failure would not say which arm broke.'
severity: minor
resolution: 'Added `resolves the read-only cursor for the display arm`, which injects the rules with the real `[data-v-x]` scope suffix and asserts computed cursor for all three arms — the assertion that actually catches RR-CBC1XZ (mutation-verified: it fails with `expected ''not-allowed'' to be ''default''`). Rewrote the hook test''s comment to say plainly what it does and does not cover, and pointed it at the cascade test. Converted the loop to `it.each`, so each arm reports separately.'
status: addressed
---

Both tests are kept deliberately: they fail for different reasons. The hook
test catches a careless template refactor that drops the class; the cascade
test catches a stylesheet edit that makes the class inert. Neither subsumes
the other.

The cascade test duplicates the two selectors from the SFC, which is a real
maintenance cost — noted in the test comment so the next editor changes both.
Vitest does not apply scoped SFC styles, so there is no way to assert this
against the component's own CSS without a browser-based runner.
