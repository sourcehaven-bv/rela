---
id: RR-BA1NIV
type: review-response
title: 'bypassACL vs field gate: plan asserts a half-elevation the codebase explicitly rejects'
finding: 'The plan proposes (without argument) that bypassACL does NOT bypass the field gate. The codebase points the other way. bypassACL is the elevated write handle for rela.bypass_acl (manager.go:66-76) and authorizeAndAudit returns nil immediately on that path (manager.go:266-269). The companion READ elevation is total: appbuild/elevatedreads.go:22-25 grants visibility.Unrestricted, and lua/deps.go:126-133 explains why partial elevation is wrong — ''Reads through it are unredacted and ungated — the closure is the boundary, so a half-elevated read would be a confusing contract... Elevation that quietly degrades to the caller''s view is worse than a loud failure.'' The field gate is field-level ACL derived from the same principal-scoped resolver. A bypass_acl closure that can write any entity but silently cannot set a read-only-for-this-principal field is precisely the half-elevated contract that comment rejects, and it is undiagnosable: the author asked for elevation and got most of it. Either bypass the field gate too (consistent, audited via the existing recordACLBypass at manager.go:267), or keep the proposal and document why field policy is categorically different from row policy — pinned by a test. The plan currently offers neither.'
severity: significant
resolution: |-
    RESOLVED: elevation is TOTAL — bypassACL bypasses the field gate too, with recordACLBypass still firing.

    Decided in light of the project's config-is-public posture (root CLAUDE.md: 'The configuration is not a secret; the data is'). An operator authoring rela.bypass_acl(...) can already read acl.yaml in full, so a field gate that blocks them conceals nothing from them — they know precisely which field is gated and by which rule. It is not a confidentiality boundary in that context, only a silent obstacle to route around. That is exactly the 'half-elevated contract' lua/deps.go:126-133 rejects: 'Elevation that quietly degrades to the caller's view is worse than a loud failure.'

    This also restores symmetry with the read side, where elevation grants visibility.Unrestricted (appbuild/elevatedreads.go:22-25) with no partial variant.

    UNCHANGED: recordACLBypass (manager.go:267) must still fire for the field-gate bypass. Audit is about attribution, not secrecy — it is orthogonal to the config-is-public argument and survives it. Pin with a test asserting an elevated patch of a gated field (a) succeeds and (b) emits the bypass audit record.
status: addressed
---

## Evidence

`internal/lua/deps.go:126-133` — the elevation contract:

> Reads through it are unredacted and ungated — the closure is the boundary, so
> a half-elevated read would be a confusing contract... Elevation that quietly
> degrades to the caller's view is worse than a loud failure.

`internal/entitymanager/manager.go:266-269` — `authorizeAndAudit` short-circuits
on the bypass path, recording via `recordACLBypass`.

`internal/appbuild/elevatedreads.go:22-25` — the read side grants
`visibility.Unrestricted`, i.e. total.

## Recommendation

Prefer **bypassing the field gate too**, for symmetry with the read side, with
the bypass recorded through the existing `recordACLBypass` channel so it stays
auditable. `rela.bypass_acl` is already an explicitly-requested, audited,
closure-scoped capability; making it total is the honest contract.

If the opposite is chosen, the plan must state *why* field policy differs from
row policy and pin it with a test, so the next reader does not "fix" the
inconsistency.

Note `elevated()`/`gated()` copy `m.deps` wholesale (`manager.go:82-107`), so
the gate is carried automatically either way — the decision is purely about
whether `PatchEntity` consults `m.bypassACL` before calling it.
