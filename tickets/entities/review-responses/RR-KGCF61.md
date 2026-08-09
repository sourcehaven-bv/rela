---
id: RR-KGCF61
type: review-response
title: AC3 contradicts createFormForType's edit-mode fallback
finding: 'The plan claims createFormForType returns "" for a type with only an edit-mode form, and AC3 asserts no affordance in that case. views_handler.go:684-707 does the opposite: its doc comment says it "falls back to edit-mode forms (which work for creation when no entity ID is provided)" and it returns `fallback`. AC3 would fail on first implementation, and "fixing" the resolver would regress its existing caller sections.go:394 (side-panel add targets).'
severity: critical
resolution: 'Condition B redefined to match the existing resolver (any usable form, edit-mode included as fallback). AC3 rewritten to test the real boundary: no form at all → no affordance; edit-only form → affordance using that form. createFormForType reused unchanged so sections.go:394 is untouched, plus a three-shape table test pinning its behaviour.'
status: addressed
---

## Resolution

Condition B is defined as **an entity type that has any usable create form**,
matching the existing resolver — an edit-mode form is usable for creation when
no entity id is supplied, which is exactly the nested-create case.

AC3 is rewritten to test the real boundary: **a type with no form at all**
(`createFormForType` returns `""`) shows no affordance. A type with only an
edit-mode form does get the affordance, using that form.

`createFormForType` is reused unchanged, so `sections.go:394` is untouched.
Added to the test plan: a table-driven test in `views_handler_test.go` pinning
the resolver's three shapes (create-only / edit-only / both), so a future "fix"
to the fallback fails loudly rather than silently breaking side panels.
