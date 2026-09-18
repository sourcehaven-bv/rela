---
id: RR-JF9QWK
type: review-response
title: Scan re-walks src/ on every run
finding: The scan walks the whole `src/` tree with a synchronous `statSync` per entry, once per run. Fast enough today, but it runs on every invocation of the test file.
severity: nit
reason: The reviewer's own assessment was 'not worth fixing, just don't let it grow', and I agree. Measured at ~0.5s for the whole file including the two other assertions, against a 19s full-suite runtime. There is direct in-tree precedent for the same walk in styles/focusRing.test.ts, styles/markdownContentMirror.test.ts and others, so caching here would diverge from the established pattern for no measurable gain. Revisit if the suite ever becomes walk-dominated.
status: wont-fix
---

Finding 7 from the cranky-code-reviewer.
