---
id: RR-FB1I
type: review-response
title: 'S5: writable: boolean discards option-verdicts (affordance regression)'
finding: |
  FieldAffordance is `{ writable?: boolean; options?: Record<string, boolean> }`. Flattening to `writable: boolean` loses `options`, which drives per-option disable in SelectWidget. Cells with allowed-options gating would show all options enabled — a regression vs DynamicForm.
severity: significant
status: addressed
resolution: |
  PLAN's `fields` prop carries the full `verdict?: FieldAffordance` (see RR-FB1H). SectionEditForm derives `writable` from `verdict.writable !== false` and `optionVerdicts` from `verdict.options`. Mirrors DynamicForm's `isFieldReadonly` + `optionVerdictsFor` helpers (DynamicForm.vue L185-199); extract into a shared utility `frontend/src/utils/affordances.ts` so SectionEditForm + DynamicForm reuse identical logic.
---
