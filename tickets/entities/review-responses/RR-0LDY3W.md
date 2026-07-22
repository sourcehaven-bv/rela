---
id: RR-0LDY3W
type: review-response
title: available_on is not enforced at exec; docs should say so explicitly now that authorization is documented next to it
finding: |-
    resolveCommands filters on matchesPage (commands.go:132) but handleCommandExec never checks page scope — it dispatches purely on cmd.Context (commands.go:383). So a command scoped via `available_on: {lists: [tickets]}` is executable from anywhere by direct POST with any list_id.

    This does NOT create resolve/exec drift in the authorization verdict: both call authorizeCommand, which does not consider page scope, so they agree on allow/deny. The drift is only that resolve additionally hides buttons that exec would still run. That is the safe direction (no false-allow in the UI), so it is not a bug.

    It is a documentation gap, and this ticket sharpens it: docs/data-entry.md now has an Authorization section sitting directly above the available_on documentation. A reader encountering `available_on` next to `permission` will reasonably infer that both restrict. One does; the other is presentation only.

    This is the same substance as RR-L6UXCF (filed against TKT-72SCPR, deferred to this ticket), whose fallback obligation was: 'If exec-time enforcement is rejected, the fallback obligation is to document that available_on is display scoping only.' Exec-time matchesPage enforcement was NOT implemented in this PR, so that fallback obligation is now due.

    ACTION: one sentence in docs/data-entry.md under available_on — 'available_on controls where a button appears; it is not an authorization boundary. Use permission: to control who may execute a command.'
severity: minor
resolution: |-
    FIXED. docs/data-entry.md now carries an explicit callout in the Authorization section, directly above the available_on documentation:

      'available_on is not an authorization boundary. It controls where a button appears; it is not checked at execution time, so a command scoped to one list or entity type can still be invoked directly against any other. Use permission: to control who may run a command, and note that a command reads whatever its context assembles from the caller-supplied entity_id / list_id — not from the page the button was on.'

    It cross-links to the new 'What a command permission actually confers' section in docs/acl-security.md (RR-37AYC0), so the two halves of the mental model — where a button shows vs. what a grant confers — are connected rather than discoverable separately.

    This discharges the fallback obligation recorded on RR-L6UXCF: exec-time matchesPage enforcement was not implemented in this PR, so the documentation alternative was owed. The claim that available_on restricts is no longer left for a reader to infer from its adjacency to permission:.
status: addressed
---

Cheap to close and it discharges an obligation already accepted on RR-L6UXCF.
Recommend fixing in this PR rather than deferring again.
