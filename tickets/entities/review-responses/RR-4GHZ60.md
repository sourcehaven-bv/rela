---
id: RR-4GHZ60
type: review-response
title: 'PR-B: three grants the linter failed to explain, and an untested alias claim'
finding: |-
    Four findings from the PR-B code review, all verified by execution before fixing. None is a security hole — every gap fails closed — but three are the linter failing at the one job it has: explaining a silent denial to an operator who cannot see why their grant does nothing. A linter that misdiagnoses is worse than no linter, because now someone trusts it.

    1. `*@draft` produced ZERO findings while granting nothing. Verified: acl.GrantsVerbOnState honors `*` only for the default state, so `*@draft` matches an entity whose type is literally named `*`. B1 skipped it as a wildcard; B11 skipped it as an undeclared type. An operator writing it reasonably believes they granted draft-write across every type.

    2. `world:Default` (any case variant) was flagged with a fix the loader REFUSES. Verified the emitted text: 'declare world "Default" under `worlds:`' — but validateWorlds rejects any name case-folding to "default" as reserved. The operator follows the advice, gets a schema load failure, and now has two problems. The grant IS dead (roleGrantsWorldRead compares case-sensitively), so flagging it was right; only the remedy was wrong.

    3. On a policy that skipped Validate, RoleDef.Worlds is unpopulated, so B10 examined nothing and B1 instead reported the world token as an undeclared ENTITY TYPE — misleading advice at High severity, which gates CI.

    4. metamodel.HasWorld/HasFace had NO direct tests. Every assertion ran against fakeMetamodel, whose HasFace is a different implementation with no alias handling at all — so HasFace's doc claim 'resolves aliases through GetEntityDef' was asserted by a comment and verified by nothing.
severity: significant
resolution: |-
    1. B11 special-cases the type wildcard before the HasEntityType gate, with a fix naming both real options (name each type, or drop the `@state`). Pinned by TestB11_TypeWildcardWithState, which also asserts a bare `*` is NOT flagged.

    2. New `defaultWorldCaseVariant` helper, shared by the role-grant and ceiling paths, emitting a message that points at the lowercase spelling instead. TestB10_DefaultWorldCaseVariant asserts the fix does NOT say 'declare world "Default"'.

    3. B10 now also scans Read for residual `world:` tokens and B1 skips them, so the diagnosis is right either way and there is exactly one finding. Audit's godoc states the precondition regardless. Pinned by TestB10_DiagnosesUnvalidatedPolicy.

    4. Added internal/metamodel/worldface_test.go. Writing it caught that the alias claim needs InitAliases() — a hand-built Metamodel literal has no aliasMap, so ResolveAlias is a no-op and the case would have passed vacuously. Now genuinely verified.

    Also: collapsed grantEntityType onto splitStateGrant (one splitter), fixed a comment naming a function that never existed, and reworded B11's fix to note that adding a `faces:` block to satisfy an ACL typo is a schema change with real semantic consequences.
status: addressed
---
