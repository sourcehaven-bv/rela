---
id: DEC-EIHQSU
type: decision
title: Command authorization fails open under NopACL, fails closed under a configured acl.yaml
context: |-
    Data-entry commands (data-entry.yaml `commands:`) execute arbitrary shell via `sh -c` and today have no authorization whatsoever. Verified: grep for acl/ACL/translateVerb/WriteRequest/Principal across internal/dataentry/commands.go and command_handler.go returns zero matches; handleCommandExec (commands.go:281) checks HTTP method, looks up the command ID, builds stdin, and execs. The only gates are requireSameOrigin/requireLocalHost (router.go:105-108), which are CSRF and network-location controls, not authorization.

    TKT-MJ02AO adds a per-command `permission:` key checked against the ACL, following the acl.PermHistoryRead precedent (internal/acl/policy.go:43) — a global named permission granted via a role's `permissions:` list, checked with gate.HoldsPermission. That pattern fits because a command has no entity Subject: acl.Subject is a sealed sum (internal/acl/subject.go:20) of EntitySubject and RelationSubject only, and `context: global`/`context: list` commands act on the project or a type-set rather than a row.

    That leaves the question this decision answers: what happens when `permission:` is omitted? Every existing deployment has zero `permission:` keys, so the default determines whether the feature protects anyone by default and whether it is a breaking change.
consequences: |-
    POSITIVE:
    - No behavior change for any deployment without an acl.yaml. A project with no policy configured runs exactly as today, so the change is invisible to users who have not opted into access control.
    - An operator who HAS opted into access control does not silently retain an ungated shell-exec surface. Adding acl.yaml means commands are governed, which matches the intent of writing a policy at all.
    - The failure mode under policy is a denied command (visible, debuggable via the Decision's RuleKind/RuleID) rather than an unguarded one.
    - resolveCommands filters unexecutable commands out of GET /_commands, so buttons for denied commands never render — the UX degrades to 'absent', not 'present but broken'.

    NEGATIVE / COSTS:
    - Breaking change at the moment a deployment adds its first acl.yaml: every command needs a `permission:` key and a matching grant, or it stops working. Must be release-noted.
    - view-context commands stop working entirely under a configured acl.yaml until view support lands (see the view carve-out below). For a project that relies on view commands, adopting acl.yaml is blocked, not merely inconvenient.
    - The rule is bimodal, so behavior differs between an unconfigured dev project and a configured production one. Tests must cover both halves explicitly.

    VIEW CARVE-OUT: `view` is not an exception to the fail policy — it is an exception to fine-grained control. Under acl.yaml, view commands are denied outright with no `permission:` escape hatch. Rationale: executeView (views.go:19-49) runs a multi-pass graph traversal and buildViewInput (commands.go:181-215) hands the script the whole traversal closure (Collections + inter-relations) with no read-gate scoping, so a `permission:` grant on a view command would confer read access whose blast radius an operator cannot determine from the config. Denying is preferable to shipping a grant with unclear scope. validateCommands warns if `permission:` is set on a view command, since the key is not honored.
date: "2026-07-20"
status: accepted
---

## Decision

Command execution authorization uses a **bimodal default**, uniform across all
four command contexts:

| ACL state | `permission:` set | `permission:` omitted |
|---|---|---|
| `NopACL` (no `acl.yaml`) | grant checked | **allowed** (fail-open) |
| `acl.yaml` present | grant required | **denied** (fail-closed) |

Per context under a configured `acl.yaml`:

| context | `permission:` set | omitted |
|---|---|---|
| `entity` | grant required | denied |
| `list` | grant required | denied |
| `global` | grant required | denied |
| `view` | **key not honored** | denied |

Under `ReadOnlyACL` (`rela-server --read-only`), **all** command execution is
denied regardless of `permission:` or context. That is independent of this
decision — it closes RR-CWWJGW, where read-only currently fails to block command
exec because `ReadOnlyACL` only implements `AuthorizeWrite(WriteRequest)` and
command exec constructs none.

## Alternatives rejected

**Fail-open everywhere (omitted → always allowed).** Preserves every deployment
and is never breaking, but the guard would protect only operators who know to
opt in per command. An operator who writes an `acl.yaml` and carefully restricts
entity writes would still have an ungated `sh -c` endpoint, which is the exact
gap this work exists to close. Rejected as security theater.

**Fail-closed everywhere (omitted → always denied).** Safe, but breaks every
existing deployment immediately on upgrade, including projects with no interest
in access control. The cost lands on users who never asked for the feature.
Rejected as disproportionate.

**Per-context defaults** (e.g. fail-open for `entity`, fail-closed for
`global`). Considered because `global` has the widest reach. Rejected: a rule an
operator cannot state in one sentence is a rule they will misconfigure. The view
carve-out is deliberately framed as "no fine-grained control", not "a different
fail policy", to keep the single rule intact.

## Implementation

TKT-MJ02AO. See that ticket for enforcement points (`handleCommandExec` before
the context switch at `commands.go:301`; `resolveCommands` at `commands.go:47`
for the cosmetic filter) and the full acceptance criteria.
