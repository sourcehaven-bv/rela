---
id: TKT-0NODIZ
type: ticket
title: 'Access skeleton: mode decisions and request lifecycle'
kind: refactor
priority: medium
effort: m
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

## Description

Second PR of TKT-WFS778. It creates the package `internal/access` and moves two
things into it: the decisions that depend on which ACL mode is configured, and
the opening of the per-request ACL state. It also defines `acl.Verb` as `acl.Op`
plus read.

The golden corpus from TKT-0M4H0S must stay byte-identical. No endpoint changes
behaviour.

## Scope

In scope:

- New package `internal/access` with a package doc that names the end state
of TKT-WFS778.
- The five closed type switches on the ACL implementation become methods of
one `access.Mode` value, with their arms unchanged: `permitsGatedUIElement`
(views_handler.go), `authorizeElevatedDocument`
(standalone_document_handler.go), `revealIsPrivileged` (history_handler.go),
`aclPolicyActive` and `commentWritesPermitted` (comments_wiring.go), and the
read-only arm of `SelectCommandAuthorizer` (commands.go).
- Request lifecycle: the body of `attachACLRequest` (reuse of an attached
request, principal resolution, `ForPrincipal`) moves to `access.Opener.Open`,
and the identity part of `maybeProvision` (re-resolve, re-stamp, `ForPrincipal`)
to `access.Opener.Restamp`. One `acl.Request` is opened per HTTP request and
every later read uses it. The `http.Handler` shell, `isAPIPath`, the error
responses and stub creation stay in dataentry.
- `acl.Verb` is defined from `acl.Op`: every write `Op` is a verb, and read
is the one extra verb. The access reports (`rela acl can`, `who-can`, `map`)
take their verbs from `acl.Verbs()` minus rename, so a new write `Op` reaches
them. Rename stays out until TKT-4APGVF makes the report helpers answer it as
the runtime does; the report output does not change.
- `.go-arch-lint.yml` gains the `access` component.

Out of scope (later PRs of TKT-WFS778):

- Moving `readGate` implementations, the gated reader, write authorization
and affordance verdicts (PRs 3 to 5).
- The other `a.acl.(*acl.Declarative)` assertions that fetch the policy
rather than decide on a mode.
- Denying dataentry -> acl and dataentry -> affordances (PR 6).

## Notes

- Design review: `access.Mode` replaces the design's `Writer` for the five
switches. PR 6 obtains the `Mode` from the access service rather than calling
`ModeOf` on an `acl.ACL`, so the arch-lint deny can hold.
- Found in review, filed separately: BUG-AR6NN9 (the provisioned principal
loses its client ceiling) and TKT-4APGVF (rename in the access reports).
- Cut from the design's PR 2: nothing beyond the rename report change above.
The read gate implementations stay in dataentry until PR 3, as planned.
