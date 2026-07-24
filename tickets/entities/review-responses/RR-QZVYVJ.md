---
id: RR-QZVYVJ
type: review-response
title: map[string]string forecloses one-claim-to-many-roles; EveryoneRole as target double-attributes
finding: 'The plan''s AssertedRoleAssignments map[string]string allows one role per claim value. Iterating principal.AssertedRoles and looking each up does handle the multi-role user correctly (dedup comes free from the seen[attrKey] guard at acl/resolver.go:22-28), but the realistic operator policy `admin -> [editor, auditor]` is foreclosed, and acl.yaml is a published schema so widening later is breaking. map[string][]string costs nothing now. Separately: if a claim maps TO EveryoneRole as the target, computeGlobals adds it with Source{Kind: SourceAsserted} while the existing block at resolver.go:45-47 adds it with Source{Kind: SourceGlobal} — different attrKey, so the same role is attributed twice and double-reports in aclmap and 403 diagnostics. Also unaddressed: claim values differing only by case or leading/trailing whitespace silently never match, which is a policy that grants nothing and an operator will not notice.'
severity: critical
resolution: 'Resolved with the user. Underlying type is map[string][]string (many roles per claim), with a scalar-or-list UnmarshalYAML so `admin: editor` and `admin: [editor, auditor]` both parse. This matches an idiom already in-tree: metamodel.StringOrSlice (internal/metamodel/types.go:696-714) does exactly this — try string, wrap in a slice, else try slice. internal/acl cannot import metamodel (arch-lint), so acl gets its own small equivalent type; metamodel itself carries several such local YAML types, so this is precedented rather than duplication-for-its-own-sake. The EveryoneRole-as-target collision is resolved by rejecting EveryoneRole as a mapping target in Validate (cleaner than accepting and pinning the duplicate attribution). Claim-value matching is exact after TrimSpace, no case folding — pinned by test, and Validate rejects a key that is blank after trimming so a silently-never-matching policy fails loudly at load.'
status: addressed
---

## Finding

Two problems with the claim→role mapping shape.

**1. `map[string]string` is a one-way door.**

The ticket's own ground-truth section documents that a Pratique user routinely
holds several roles (`["admin","billing"]`). Iterating `AssertedRoles` and
looking each up handles that correctly — and the `seen[attrKey]` dedup at
`acl/resolver.go:22-28` handles duplicate claim values for free. **The plan is
right that multi-role works.**

But it is one role *per claim value*. The policy an operator will actually want
is `admin → [editor, auditor]`, and `map[string]string` forecloses it.
`acl.yaml` is a published schema, so widening later is a breaking change.
`map[string][]string` costs nothing today.

**2. `EveryoneRole` as a mapping target double-attributes.**

If a claim maps to `EveryoneRole`, `computeGlobals` adds `add(EveryoneRole,
Source{Kind: SourceAsserted, Claim: ...})` while the existing block at
`resolver.go:45-47` already added `add(EveryoneRole, Source{Kind:
SourceGlobal})`. Different `Source` ⇒ different `attrKey` ⇒ **two attributions
for one role**, which double-report in `aclmap` and in 403 diagnostics.

**3. Whitespace and case are unspecified.**

`Validate` gains only a non-blank-key check per the plan. A claim value
`"Admin"` vs `"admin"`, or `" admin"`, silently never matches — a policy that
grants nothing, which is exactly the failure an operator does not notice.
`isBlank` (`policy.go:544`) already exists as a precedent.

## Resolution

- Choose `map[string][]string` unless there is a reason not to; document the
choice in `docs/acl-overview.md`.
- Reject `EveryoneRole` as a mapping target in `Validate`, or accept it and pin
the duplicate with a test. Rejecting is cleaner.
- Decide trim/case-fold semantics explicitly and pin with a test.
