---
id: RR-UD2H
type: review-response
title: Edit-mode tests received only mechanical mode:'edit' additions
finding: |
  Every existing edit-mode test got mode: 'edit' as const mechanically. None were re-examined for what they assert. Most assert structural shapes (find('input').exists()) which would still pass even if the widget broke functionally. No new edit-mode test asserts that the display branch is NOT rendered in edit mode (the dual v-if/v-else is one of the easiest mistakes to make in a future widget).
severity: minor
status: addressed
resolution: |
  Added one negative assertion to each widget's canonical edit-mode test: expect display branch is NOT rendered. Text/Textarea/Number/Date assert `span.display-value` doesn't exist; Checkbox asserts no `.display-checkbox` and that the input isn't disabled; Select asserts no Badge is rendered; MultiSelect asserts no `.badge-row`; Rrule asserts no display-value span. Catches v-if/v-else inversion mistakes in any future widget edit.
---
