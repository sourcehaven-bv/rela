---
id: BUG-197XPAU
type: bug
title: Conflict resolve skips field rights and errors quote hidden values
description: 'Conflict resolve skips the field write check; validation messages quote values; git status lists ungated conflict paths (issue #2098).'
priority: medium
effort: m
why1: Conflict resolve ran only the row write check. Validation messages quoted the rejected value. Git status and sync listed every conflicted path to a git:sync holder.
why2: The conflict routes read files outside the store, so the field check in the update path and the read gate in visibility were not on their way. Messages were written for the CLI author, who sees every value.
why3: BUG-T6HNKS accepted git:sync as the only gate for conflicts, so later work added row gating route by route and the field check and status paths were left behind.
why4: A path that reads project files outside the store has no default read gate and no default field check. Each surface must remember both.
why5: Write-side field rules and read-side row gates live in the store-backed paths. Code that bypasses the store re-implements them by hand and the review checklist does not ask which ones a new surface skips.
prevention: Resolve runs the PATCH field check through conflictFieldDenial. Status and sync share readableConflictPaths with the conflict list. Built-in validation messages never quote values and TestValidatePropertyValue_MessageNeverQuotesValue pins every type. Handler tests pin each surface.
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

## Description

Three gaps around the conflict routes, found in the CISO review of #2097 (GitHub
issue #2098).

1. `POST /_conflicts/resolve` runs the row write check but no field write
check. A principal with `git:sync` and `update` can pick, through
`property_choices`, the value of a field that `writable:` denies or `visible:`
hides.
2. A validation error from resolve goes to the client verbatim. The metamodel
message quotes the offending value (`Invalid value "x" (allowed: ...)`), so an
invalid value in a `visible:`-hidden field leaks past redaction. The normal
update path has the same leak: soft validation findings ride on a 200 as
warnings whose detail is the same message.
3. `GET /_git/status` and a failed `POST /_git/sync` list every conflict path
to every `git:sync` holder, ungated. `/_conflicts` leaves out the files that
name a hidden row. `docs/acl-security.md` still says conflict routes are not
row-gated.

## Expected

1. A choice for a field the principal may not write is refused like the same
field in a PATCH.
2. No error or warning carries a property value.
3. The status and sync responses list only the conflict paths `/_conflicts`
lists.
