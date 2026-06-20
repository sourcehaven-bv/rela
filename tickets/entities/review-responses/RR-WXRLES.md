---
id: RR-WXRLES
type: review-response
title: ApplyEntity existence probe was fail-open (wrong ACL/audit op on transient error)
finding: 'apply.go existence check `exists := getErr == nil` treated EVERY non-nil GetEntity error (pgstore network blip, fsstore IO/parse failure, context.Canceled) as ''does not exist'' → op=OpCreate. Consequences: (1) ACL bypass — create/update are separately grantable (acl/policy.go grantsVerb); an update-only principal hitting a flaky existence read gets authorized as Create. (2) audit corruption — ''create'' rows written for updates during exactly the backend incident you''d investigate. The same package''s RenameEntity (manager.go:628-639) already establishes the fail-closed pattern (errors.Is(getErr, store.ErrNotFound) else return error); ApplyEntity ignored it.'
severity: critical
resolution: 'Added resolveUpsertOp(getErr,...) helper that fails CLOSED: nil→update, store.ErrNotFound→create, any other error→return it (mirrors RenameEntity). Used by both ApplyEntity and ApplyRelation. Regression TestApplyEntity_ExistenceProbeFailsClosed (flakyProbeStore returns a transient error) asserts the error propagates instead of becoming a create. resolveUpsertOp at 100% coverage.'
status: addressed
---
