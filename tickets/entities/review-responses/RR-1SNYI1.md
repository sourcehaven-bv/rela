---
id: RR-1SNYI1
type: review-response
title: isLongValue's KEEP IN SYNC comment overclaimed; inert-render warning didn't name the field index
finding: 'Two small precision defects. (a) `isLongValue` (SectionEditForm.vue) carried a ''KEEP IN SYNC with PropertyDisplay.vue isLong'' comment, but PropertyDisplay also short-circuits on `PropertyItem.isLongText` and uses `|| ''''` where the port uses `?? ''''` — so the comment claimed a byte-identical port that wasn''t one. (b) `inertSectionRenderWarnings` always said ''section[%d] sets render:'' even when a FIELD carried the setting, leaving an operator with a long fields: list no face — while the error path (validateSectionRender) is precise to field[%d].'
severity: nit
resolution: '(a) Narrowed the comment to state exactly what is shared (the 60-char threshold) and to record that `isLongText` is never populated anywhere in the codebase — including in PropertyDisplay itself — so there is no behavioural difference to mirror, with a note to add the arm if it is ever wired up. (b) The warning now names the precise origin: `section[i]` for a section-level render, `section[i] field[j]` for the first offending field. Pinned by TestCollectConfigWarnings_InertSectionRenderNamesOrigin.'
status: addressed
---

On (a): confirmed by `grep -rn "isLongText" src/` that the only occurrences are
its declaration and read inside `PropertyDisplay.vue` — nothing ever sets it. So
the two functions are behaviourally identical today and the `|| ''` vs `?? ''`
difference is unreachable at a 60-character threshold. The fix is to the
comment, not the code.

The reviewer also independently verified the reactivity question I had not
explicitly tested — that Vue 3's proxy tracking follows through a plain helper
called inside a computed, including for keys added to `formData` after first
read. Not a bug.
