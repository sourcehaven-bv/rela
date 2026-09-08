---
id: BUG-R9GOCK
type: bug
title: Copy read gates are silently nil-tolerant under a compiled ACL policy
description: 'Deps.CopyReadGate and Deps.CopyVisibility had no non-nil check in entitymanager.New, unlike the eight other collaborators in the same struct. With a compiled acl.yaml wired but the two gates forgotten, authorizeCopy skips its source READ verdict and readCopySource reads cross-entity sources raw — bypassing visible: field redaction, with no error and no log line. The appbuildtest fixture sat in exactly that state for any caller passing WithACL(declarative). Reported as GitHub issue #1437 from an IB-review of PR #1431.'
priority: high
effort: s
why1: CopyReadGate and CopyVisibility were declared in Deps with no non-nil check in New, so a wiring site that omitted them produced a Manager whose copy path read its source ungated and unredacted, with no error and no log line.
why2: Their nil-tolerance was justified by the no-policy case (a CLI deployment with no acl.yaml, where every read is raw anyway), but nothing enforced the condition the justification rested on — that nil actually MEANS no policy. The assumption lived only in a code comment.
why3: 'The obvious guard, gating on d.ACL != nil, is vacuous here: ACL is itself required a few lines above, so it is never nil. Expressing the real condition needs a different predicate (the ACL being a compiled *acl.Declarative), which is more thought than a nil check, so the check was skipped rather than adapted.'
why4: The sibling deps hid it. FieldGate right above carries a godoc explaining that a silently-nil authz gate is the forgotten-wiring ACL bypass (RR-X9NVHI), and CopyGuard beside it already fails closed. Two of the three copy deps were safe, so the pair that was not looked consistent with its neighbours at a glance.
why5: There is no invariant that an authorization dependency must fail fast, so each is given whatever strictness its author considered. The rule was recorded as prose on individual fields (FieldGate's godoc, CLAUDE.md's constructors reject nil required fields) rather than as an enumeration over the struct, so a later-added pair of gates inherited the prose without inheriting the check.
prevention: 'entitymanager.New now refuses a policy-backed Deps (ACL is *acl.Declarative) whose CopyReadGate or CopyVisibility is nil, naming only the missing one, with AllowAllCopyReadGate / AllowAllCopyVisibility as the explicit opt-outs. Both gates moved into the shared appbuild.TransitionWiring bundle so production and the test fixture build them from one source rather than by hand-copy — which is what let the fixture drift into the unsafe state. Pinned by TestNew_CopyGatesRequiredUnderPolicy (both tiers, so the no-policy CLI case cannot regress either) and TestCompileTransitions_AlwaysSuppliesCopyGates at the wiring layer. The broader lesson, not yet mechanised: an authorization dependency added to an existing Deps struct should be checked against the struct''s other authz fields rather than against prose, since the prose does not enumerate the fields it governs.'
status: done
---

## Symptom

`entitymanager.Deps` requires eight collaborators — `Store`, `Meta`,
`Templater`, `Audit`, `ACL`, `Transitions`, `Computed`, `FieldGate` — each with
a non-nil check that makes `New` fail fast. Two more in the same struct had no
such check:

```go
CopyVisibility CopyReader
CopyReadGate   CopyReadGate
```

With a policy wired and these two left nil, the copy path degrades silently:

- `authorizeCopy` skips check (1), the READ verdict on the copy's source
(`internal/entitymanager/copy.go:421`), and
- `readCopySource` takes the raw-store branch for cross-entity copies
(`internal/entitymanager/copy.go:362`), bypassing `visible:` field redaction.

No error, no warning, no log line. The only marker was a comment asserting an
assumption that nothing enforced: *"A nil gate means no ACL is wired, which is
the CLI/no-policy case."*

## The risky state existed; it was not hypothetical

`appbuild.buildEntityManager` wires all three copy deps correctly. The
`appbuildtest` fixture hand-copies that `Deps` literal and left both read gates
nil, reasoning that its default ACL is `NopACL`. But the fixture accepts
`WithACL(...)`, so a caller passing a real declarative policy got a
policy-backed manager whose copy path read its source ungated and unredacted.

Adding the guard failed those tests immediately — which is the evidence the
condition was reachable rather than theoretical. The fixture's own comment had
already predicted the mechanism:

> NOTE: this Deps literal is a HAND-COPY of buildEntityManager's, not a call to
> it, so the two drift silently […] That is how the Copy* deps came to be
> unwired in BOTH places at once.

## Fix

`New` refuses a policy-backed `Deps` with either gate nil, where "policy-backed"
means the ACL is `*acl.Declarative` — the same test the wiring already applied.
The no-policy tier (`NopACL` / `ReadOnlyACL`: CLI, demos) is untouched. Opting
out remains possible but must be stated: `AllowAllCopyReadGate{}` /
`AllowAllCopyVisibility{Store: …}`.

Both gates moved into the existing `TransitionWiring` bundle, whose stated job
is that "every wiring site (production assemble + test fixtures) builds the
enforcer the same way", so the two sites can no longer drift.

## Provenance

GitHub issue #1437, from an IB-review of PR #1431. That PR was closed unmerged
and the issue was triaged twice as "the code this describes is not in the tree."
That was accurate when written — the copy kernel has since landed on `develop`
by another route, so all four symbols and both fail-open branches now exist
exactly as the reporter described.
