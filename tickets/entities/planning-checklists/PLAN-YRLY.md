---
id: PLAN-YRLY
type: planning-checklist
title: "Planning: Add month/day picker for yearly rrules"
status: done
---

## Understanding

- [x] Problem: RruleBuilder widget has no month/day picker when YEARLY frequency is selected
- [x] Use case: birthdays, anniversaries — "every year on March 15"
- [x] RRULE syntax needed: `FREQ=YEARLY;BYMONTH=3;BYMONTHDAY=15`
- [x] Scope: frontend-only change to `RruleBuilder.vue` — no backend/Go changes needed

## What's NOT in scope

- Monthly BYMONTHDAY picker (separate ticket if needed)
- BYDAY positional rules (e.g., "2nd Monday of March")
- COUNT/UNTIL support

## Acceptance Criteria

1. When YEARLY frequency is selected, a month dropdown (Jan-Dec) and day input (1-31) appear
2. Selecting month+day generates correct BYMONTH and BYMONTHDAY in the rrule string
3. Parsing an existing `FREQ=YEARLY;BYMONTH=3;BYMONTHDAY=15` populates the month/day fields
4. Human-readable preview shows e.g. "every year in March on the 15th"
5. Fields are disabled in readonly mode
6. Day input clamps to 1-31

## Approach

Modify `frontend/src/components/forms/RruleBuilder.vue`:

1. Add `selectedMonth` ref (1-12, default empty) and `selectedDay` ref (1-31, default empty)
2. Add a conditional section `v-if="freq === RRule.YEARLY"` with:
   - Month dropdown (`<select>`) with Jan-Dec options (values 1-12)
   - Day-of-month number input (1-31)
3. In `parseRrule()`: extract `opts.bymonth` and `opts.bymonthday` to populate the new refs
4. In `rruleString` computed: when freq is YEARLY and **both** month and day are set, include `bymonth` and `bymonthday` in the RRule options. If only one is set, omit both (partial selection = plain YEARLY)
5. Style using existing BEM classes (`.rrule-builder__row`, etc.)

## Test Plan

- Unit: verify rrule string generation with month+day set
- Unit: verify parsing of existing yearly+month+day rules
- Manual: test in data-entry UI with a real rrule property
- Edge cases: no month/day selected (plain YEARLY), month without day (should not emit BYMONTH), day without month (should not emit BYMONTHDAY) — both must be set or neither is emitted

## Risk Assessment

- Low risk: isolated frontend change, additive only
- The rrule npm library already supports BYMONTH/BYMONTHDAY natively
- Go backend validation passes through to rrule-go which also supports these params
