---
id: RR-3LDEAK1
type: review-response
title: Undo fails for faced owned rows
finding: DELETE of ID@face is a hard face delete (write_handler.go:1177-1195) and restore takes a bare id only (softdelete_handler.go:73-78).
severity: significant
reason: Moved with the scope split (RR-CPB5PXW) to TKT-48ZHQZ3, which carries this constraint in its description. This ticket ships no row delete, unlink or Undo action, so the defect cannot occur in this change; it must be solved before TKT-48ZHQZ3 ships.
status: deferred
---
