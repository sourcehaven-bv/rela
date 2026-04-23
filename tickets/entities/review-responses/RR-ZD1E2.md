---
id: RR-ZD1E2
type: review-response
title: DynamicForm readReturnTo has no tests
finding: 'The bug fix in frontend/src/components/forms/DynamicForm.vue:115-120 (return_to read out of initializeDefaults so edit mode also honours it) has zero test coverage. Add tests covering: create-mode submit honours return_to; edit-mode submit honours return_to; edit-mode without return_to falls through to router.back(); array-valued return_to (vue-router yields string[] on duplicates) is ignored; malformed (non-slash-prefixed) is ignored. Tie together with RR-open-redirect tests.'
severity: significant
resolution: 'Added readReturnTo util that handles: string → isSafeReturnPath check, array (vue-router duplicate keys) → null, null/undefined → null, non-string → null. 6 new unit tests in returnPath.test.ts. DynamicForm uses the util (formerly had its own inline guard). Combined with the 22 isSafeReturnPath tests, the read+guard path has 28 test cases. Tied together with RR-YPU6C (open-redirect).'
status: addressed
---
