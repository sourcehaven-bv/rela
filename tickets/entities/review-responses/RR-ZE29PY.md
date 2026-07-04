---
id: RR-ZE29PY
type: review-response
title: 'CSS drift: header .section-heading duplicates EntityDetail''s, silent divergence risk'
finding: .section-edit-form-header .section-heading hand-copies EntityDetail's .section-heading; scoped styles can't cross components so a future edit to one silently diverges the Properties heading from sibling headings.
severity: significant
resolution: Added reciprocal KEEP-IN-SYNC pointer comments in both SectionEditForm.vue (.section-edit-form-header .section-heading) and EntityDetail.vue (.section-heading), each referencing RR-ZE29PY. Values confirmed matching (18px/600, var(--text-color), border via row). Lifting to shared styles deferred as beyond ticket scope.
status: addressed
---

**Finding:** `.section-edit-form-header .section-heading` hand-copies
EntityDetail's `.section-heading` (18px/600, border). Scoped styles can't cross
components, so a copy is necessary, but a future edit to EntityDetail's
`.section-heading` silently diverges the Properties heading from every sibling
heading.

**Fix:** Leave a pointer comment in both places noting they must stay in sync
(lifting to the shared `src/styles/` layer is beyond this ticket's scope).
Verify current values match.
