---
id: RR-GNXKTW
type: review-response
title: Dead bulk-PATCH branch in handleSubmit implies edit mode bulk-submits, contradicting the safety argument
finding: 'frontend/src/components/forms/DynamicForm.vue:892-900 — the `if (isEdit.value && props.entityId)` branch in handleSubmit is unreachable, because handleSubmit early-returns at line 819 when isEdit. This was harmless before, but is now actively misleading: the decision to REMOVE the withoutUntouchedRedacted guard rests on the argument that edit mode has no bulk submit. A future reader hitting this branch may reasonably conclude edit mode does bulk-submit, and reason incorrectly about whether an untouched redacted field can be clobbered. The dead branch is the last thing in the file implying otherwise.'
severity: minor
resolution: 'Deleted the unreachable branch (c1700f65) rather than annotating it — a comment would have left the misleading code in place. The create branch always returned, so the trailing post-conditional block was dead too and went with it; the create path is now unwrapped and linear. Strengthened the isEdit early-return comment to state the invariant explicitly and to instruct a future implementer of bulk edit submit to prune untouched _redacted properties first. Deletion orphaned surfaceWarnings, which exposed a pre-existing gap: the CREATE path also returns DEC-HWZHA soft warnings and never surfaced them (the only caller was the dead edit branch). Wired it into the create path instead of deleting it, so those warnings are now visible for the first time. Verified: typecheck clean, 0 lint errors, 1440/1440 frontend tests pass, Go build and tests pass.'
status: addressed
---

Found by cranky-code-reviewer on commit 9ffa06b8.

Severity is minor because there is no behavioral defect — the branch cannot
execute. It matters only because it undermines the reasoning that justified
removing a data-loss guard, and that reasoning is the kind a future maintainer
will want to re-derive rather than trust.

Deletion is safe: `DynamicForm.test.ts` pins that a form submit in edit mode
sends nothing.
