---
id: RR-6REA1Z
type: review-response
title: Validation 400 overloaded with request-shape errors
finding: TypeInvalid covers both request-shape errors and prop validation while invalid_ref is 422.
severity: nit
reason: The message store is not hand-editable, so the 200-with-warnings policy case does not apply; 400 for a refused body is consistent with the rest of the service. Ref errors stay 422 because they must match the nonexistent-ref body.
status: wont-fix
---
