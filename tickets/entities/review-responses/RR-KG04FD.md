---
id: RR-KG04FD
type: review-response
title: 'A redacted confirm field was cleared with no dialog and no consent'
finding: 'A field the principal cannot read is absent from formData, so isClearedForType read it as empty. needsConfirm skipped it (nothing to display), but clearOnHide cleared it anyway because it re-derived intent from clear_when_hidden rather than from the decision just made. Reproduced against the real component: 0 dialogs shown, properties_unset carried the redacted field. This is BUG-FB0LN8''s original symptom resurrected through the ACL path, and it violates the project rule that a redacted read must never clobber hidden fields.'
severity: critical
resolution: 'Fixed at the root by making the DECISION travel with the data: propose() computes the approved clear set and passes it to onHidden, which applies it verbatim; clearOnHide is back to yes-only. A new isUnreadable dependency makes confirm fail safe on a value it cannot display - no dialog, no clear. Pinned by tests in useChangePolicy.test.ts and DynamicForm.propose.test.ts.'
status: addressed
---
