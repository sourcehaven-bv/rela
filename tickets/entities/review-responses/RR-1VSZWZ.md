---
id: RR-1VSZWZ
type: review-response
title: State-machine entry errors give 500, and on fs leave the action stuck pending
finding: 'internal/appbuild/realtime_promote.go:112,126-160 detects nothing-written only for beforeWriteError, ValidationError and ErrEntityAlreadyExists. createCore (core.go:150) returns statemachine.ErrIllegalEntry and template or unknown-type errors before any write, unmapped, so a non-entry status answers 500 and on fs leaves the message pending. Fix: in entitymanager/linked.go:94 wrap every createCore error so WroteNothing reports true; map ErrIllegalEntry and ErrPreconditionFailed to 422 promote_invalid; add a TestPromote_ErrorClasses row.'
severity: significant
resolution: createCore errors inside the CreateLinked Tx are marked as before-write so WroteNothing is true and the shared classifier maps statemachine ErrIllegalEntry and ErrPreconditionFailed to 422 promote_invalid with the action released on every backend; illegal-entry row added to TestPromote_ErrorClasses (2ce29a3fe 6302a4b22)
status: addressed
---
