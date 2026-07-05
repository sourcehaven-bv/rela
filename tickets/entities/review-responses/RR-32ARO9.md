---
id: RR-32ARO9
type: review-response
title: Empty/falsy heading drops both <h2> and header row (bare indicator)
finding: sectionRendersOwnHeading() returns true for any properties inline-edit section without checking section.heading is truthy; when heading is empty the <h2> is suppressed AND SectionEditForm's v-if="heading" skips the header row, leaving a bare indicator with no heading.
severity: critical
resolution: 'sectionRendersOwnHeading now requires !!section.heading, keeping it in agreement with SectionEditForm''s v-if="heading". An empty-heading properties section keeps the old headless path (no <h2>, indicator via default slot). Added component test (SectionEditForm.test.ts) asserting heading:'''' → no header row, single indicator. Verified: heading case renders the header row correctly in the real app.'
status: addressed
---

**Finding:** `sectionRendersOwnHeading()` (EntityDetail.vue) returns true for
ANY properties inline-edit section without checking `section.heading` is truthy.
So the generic `<h2>` is suppressed, but `SectionEditForm`'s `v-if="heading"`
skips the `.section-edit-form-header` row when `heading === ""`, falling through
to the headless slot → a bare indicator jammed at the top-left, with no heading
at all. Reachable via a configured `data-entry.yaml` properties section with no
`heading:` (config.go `omitempty`, sections.go passes through). The guard and
the render branch disagree on what "renders own heading" means.

**Fix:** make `sectionRendersOwnHeading` also require a truthy
`section.heading`. Then an empty-heading properties section keeps the old
headless path (indicator via default slot, no `<h2>` since there's nothing to
show), and only a section WITH a heading renders the own-heading flex row.
