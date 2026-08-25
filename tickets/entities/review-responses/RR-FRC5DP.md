---
id: RR-FRC5DP
type: review-response
title: '--focus-ring-gap declared twice in the same :root block, and mirrored into the published contract file'
finding: 'The token was declared identically at tokens.css:55 and :73, with two comment blocks saying the same thing in slightly different words — the `duplication` smell the project''s commentlint rule targets, where one copy gets corrected later and the other goes stale. Harmless to render (the second simply wins), but this is a published contract file served to custom apps, and the duplicate was mirrored verbatim into apps_tokens.css. The byte-sync test was green on a duplicated file, as were all seven focus-ring guards and 1839 frontend tests.'
severity: significant
resolution: 'Removed the second declaration and its comment block. Caused by an edit landing after the file had been rewritten underneath it — the first copy was invisible in the working context at the time.'
status: addressed
---

Worth stating plainly because it is the reviewer's sharpest observation about
this ticket's test strategy: **seven purpose-built guard tests, a byte-for-byte
cross-language sync test, and 1839 frontend tests all passed on a token file
that defined the same custom property twice.** None of them model CSS as CSS;
they match text. That is the same limitation behind [[RR-FRC2GD]] and
[[RR-FRC1SP]], surfacing a third time.

A postcss-based assertion over the built stylesheet would catch all three
classes (duplicate declaration, specificity defeat, hardcoded ring colour).
Recorded as a follow-up on the ticket rather than built here.
