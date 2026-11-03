---
id: RR-0OI5LM
type: review-response
title: Promote error mapping is incomplete and contradicts dataentry conventions
finding: C4 maps only ForbiddenError, ValidationError and ErrEntityAlreadyExists, so affordances.FieldWriteError (served as 403 field-affordance by dataentry, while AC16 says 422), unique violations, customIDNotAllowed and transition guard errors all become 500. Reuse the dataentry mapping for create errors and pin each class in realtime_promote_test.go. That classification must also decide whether the revert is safe (see the critical finding).
severity: significant
resolution: The promoter reuses the dataentry create-error classification (field-affordance 403, unique and validation 422, custom id, guard). Each class is pinned in realtime_promote_test.go and decides whether the fs revert is safe.
status: addressed
---
