---
id: RR-FB2B
type: review-response
title: 'Round 2 NEW-2: affordance helper extraction silently drops field.readonly path'
finding: |
  DynamicForm's `isFieldReadonly(field)` honors BOTH `field.readonly` (static config flag) AND the `_fields[prop].writable === false` verdict. The proposed shared `isFieldWritable(verdict)` loses the static channel; refactoring DynamicForm to use it would regress form-config readonly handling. No existing test catches this regression.
severity: significant
status: addressed
resolution: |
  Shared helper signature widened: `isFieldWritable(verdict: FieldAffordance | undefined, fieldReadonly?: boolean): boolean` returning `!fieldReadonly && verdict?.writable !== false`. DynamicForm passes `field.readonly` from its FormFieldOrRelation. SectionEditForm passes `undefined` (no static-readonly concept on this surface). Tests cover both paths.

  PLAN AC 2 amended with the explicit signature.
---
