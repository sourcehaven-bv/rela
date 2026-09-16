---
id: RR-K07KLJ
type: review-response
title: Cached-badge test was vacuous and the AM claimed it was mutation-verified
finding: The test 'drops the cached badge along with the denied content' asserted the badge text was absent after a denial. The badge renders inside the `v-else-if="docContent"` branch, so blanking docContent unmounts the whole subtree and takes the badge with it — isCached could remain true forever and the assertion still passes. Deleting `isCached.value = false` left the suite fully green, so the mutant survived. AM-document-denial-blanks-content asserted the opposite under a 'Mutation-verified' heading, which made a compliance artifact for CONTROL-8-03 claim coverage that did not exist.
severity: critical
resolution: 'Investigated rather than patched, and the conclusion was that the reset line itself was unobservable: isCached is read only inside the docContent branch and is unconditionally reassigned by every successful render, so no sequence can surface a stale true. A rewritten test (denial, then access restored with an uncached render) still failed to kill the mutant, confirming this. Removed `isCached.value = false` from both components and deleted the test, leaving a comment at each site explaining why the tidier-looking line is absent. Corrected AM-document-denial-blanks-content: the false claim is replaced by a ''What this measure does NOT cover'' section recording the error, and the Mutation-verified section now carries a six-row table of individually re-measured figures.'
status: addressed
---

The reviewer's diagnosis was exact, and the general lesson is sharper than the
specific bug: an assertion can be satisfied by something other than the code it
is aimed at. Here the blanking under test also removed the evidence the test was
looking for, so the test could not distinguish the two mechanisms.

The tempting fix was a cleverer probe (reach into `wrapper.vm`, or assert after
a later render). Both were tried; neither killed the mutant, because the
behaviour genuinely is not observable. Writing an ever-more-indirect test to
defend an unobservable line would have produced exactly the false confidence
this finding is about.

The false "Mutation-verified" claim is the part that mattered most. An
automated-measure entity is what an auditor reads to believe a control is
enforced, so an overstated one is worse than an absent one.
