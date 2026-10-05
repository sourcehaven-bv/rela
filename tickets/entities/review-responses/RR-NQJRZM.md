---
id: RR-NQJRZM
type: review-response
title: Restore write errors were mapped inconsistently
finding: A restore that raced a delete or recreate surfaced as a generic 422, and a face error was not named as one.
severity: significant
resolution: writeRestoreWriteError maps ACL denial to 403, ErrEntityAlreadyExists/ErrEntityNotFound/store.ErrNotFound to 409 state_changed, face errors to 422 face_required, and everything else to the write path's 422 validation_failed, consistent with write_handler (an unknown error stays 422 there too).
status: addressed
---
