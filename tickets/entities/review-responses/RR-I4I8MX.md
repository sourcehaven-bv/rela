---
id: RR-I4I8MX
type: review-response
title: Doc comment restated resolveWorld's contract and the ordering rationale three times
finding: 'The doc on refuseWorldIncapablePath ran ~50 lines for an 11-line function. Two paragraphs were restating: one re-explained (and quoted) the denial-indistinguishability rule already stated in resolveWorld''s own doc, and the ordering rationale appeared three times in the diff — function doc, call-site comment, and the test docstring — all using the same typo example. The `duplication` commentlint rule exists for exactly this: a fact stored three times gets corrected in one place and goes stale in two.'
severity: minor
resolution: Cut the restating paragraph to one sentence that keeps the `[resolveWorld]` link rather than re-explaining through it, and removed the 11-line call-site comment in favour of a 5-line note on the guard pair. Kept the "keyed on the NAME rather than a resolved handle" paragraph in full — the reviewer agreed it earns its length, since the `handle.isDefault()` trap is non-obvious and unrecoverable from the code. `just comment-report` confirms the diff introduces no new duplication findings (the two in these files, viewworld.go:309 and worldneighbors.go:411, are pre-existing).
status: addressed
---
