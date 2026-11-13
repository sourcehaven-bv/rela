---
id: BUG-UK06LDP
type: bug
title: loadConfigPolicy drops the acl.yaml validation error silently
description: Under an injected ACL loadConfigPolicy discards the ValidateACLPolicy error with a blank assignment; IMPL-RXQ0PI1 still ticks No silent failures (GH-2226).
priority: low
effort: xs
why1: loadConfigPolicy assigned the ValidateACLPolicy error to the blank identifier.
why2: The tolerance for a schema-mismatched acl.yaml was implemented as discard instead of as log and continue.
why3: The implementation checklist ticked No silent failures without naming the tolerated case, so review did not question the discard.
why4: The checklist item reads as all-or-nothing and gives no place to name a deliberate exception.
why5: Deliberate error tolerance has no house convention in appbuild; other tolerated startup problems log a warning (TKT-T31NKT, TKT-DOFYR1), and this one did not follow it.
prevention: A tolerated error is logged with slog.Warn; the boot subtest asserts the warning, so a return to a silent discard fails the test.
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

`loadConfigPolicy` in `internal/appbuild/realtime.go` runs `_ =
ValidateACLPolicy(p, meta)`. Tolerating a schema-mismatched acl.yaml on a host
that does not enforce it is deliberate (RR-GT629C0), but the error was neither
logged nor returned, and IMPL-RXQ0PI1 ticked "No silent failures" without naming
the exception. Reported as GitHub issue #2226.

## Fix

Log the validation error with `slog.Warn` and keep booting. The subtest
"acl.yaml that does not fit the schema still boots" asserts the warning.
IMPL-RXQ0PI1 names the exception.
