---
id: RR-MNYZCY0
type: review-response
title: Response mapping for relation errors and BatchKeptError
finding: ClassifyCreateError is the entity classifier; relation failures and BatchKeptError (raw, unredacted entities) need an explicit mapping.
severity: significant
resolution: writeBatchError maps BatchKeptError to a bare 500, ErrBatchInvalid to 400, ForbiddenError to 403 with rule_kind and rule_id, face to 422 face_required, conflict to 409, a missing or hidden end to 422 target_not_found at /relations/j/{from|to}, a type-disallowed edge to its code, and other invalid errors to 422.
status: addressed
---
