---
id: RR-DDY9LG
type: review-response
title: Commit filter silently drops untouched prefilled properties
finding: 'visibleWritablePropertiesForCommit (DynamicForm.vue:491-512) keeps a key unconditionally only when it is in userTouched; otherwise it omits any key the create dry-run reports as hidden or read-only. applyTemplate (:935-955) writes formData directly and never adds to userTouched, so every prefilled-but-untyped property is subject to that drop. The user sees the value in the form, submits, and the copy lacks it with no error. This is exactly the ''short copy produced silently'' outcome AC10a declares unacceptable, arriving through a channel AC10a does not cover: _redacted is server-withheld-on-READ, this is client-dropped-on-WRITE. Reusing applyTemplate inherits the bug by construction.'
severity: critical
resolution: Prefill registers its keys as userTouched, per the RR-2U2D rationale at DynamicForm.vue:497-502 (the server affordance gate is the backstop and 403s loudly). Recorded in the ticket under "The prefill marks its keys as touched"; AC19 asserts prefilled values reach the created entity.
status: addressed
---

## Resolution required

Pick one and add an AC:

1. The prefill marks its keys as `userTouched`, accepting the server's affordance
gate as the backstop. `DynamicForm.vue:497-502` states this is safe: an
under-resolved touched key that policy actually denies still 403s at commit with
a clear `rule_id`.
2. Or prefilled-but-unsubmittable properties are surfaced to the user the same
way `_redacted` ones are.

Option 1 is consistent with the existing RR-2U2D reasoning and keeps the "never
silently drop a value the user can see" invariant.
