---
id: RR-H7OWQ3H
type: review-response
title: Restore refused by owning rule returns 500
finding: writeRestoreError has no arm for ErrOwningRule (softdelete_handler.go:112-130); should map to 409.
severity: significant
reason: Moved with the scope split (RR-CPB5PXW) to TKT-48ZHQZ3, which carries this constraint in its description. This ticket ships no row delete, unlink or Undo action, so the defect cannot occur in this change; it must be solved before TKT-48ZHQZ3 ships.
status: deferred
---
