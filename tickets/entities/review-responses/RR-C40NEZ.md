---
id: RR-C40NEZ
type: review-response
title: 'PR-A code review: missing gate tests, clamp coverage guard, and three doc-vs-code mismatches'
finding: |-
    Five smaller findings from the PR-A code review, all verified:

    1. `Request.PermitsWorld` — the one new EXPORTED security gate, and the thing PR-C builds on — had ZERO tests. The load validation around it was heavily tested; the gate itself was not.
    2. `compiledCeiling.clamp` rewrites six fields, and a seventh grant axis added later and forgotten would produce no compile error and no test failure. RR-TFATPO asked for a coverage guard and it was skipped.
    3. aclaudit.go godoc cited `TestGrantEntityType_MatchesACLSplit` as pinning the duplicated grantTypeOf helper. That test did not exist — a citation to a nonexistent guard is worse than none, because the next reader trusts it.
    4. validateWorldNames rendered `invalid face "a b"` for a WORLD name — leaking a noun the operator never used.
    5. An aclaudit ceiling comment forward-referenced `B10-ceiling-undeclared-world` as though coverage existed elsewhere. No such rule exists.
severity: minor
resolution: |-
    1. Added TestPermitsWorld (6 cases: named grant, ungranted world, default via ordinary read, bare grant does not reach a non-default world, read wildcard does not either, no roles at all) plus TestPermitsWorld_CeilingOverridesRoleGrant covering the ceiling composition.
    2. Added TestClampCoversEveryGrantAxis — a reflect-based guard asserting every slice field on RoleDef is either narrowed by a permit-nothing ceiling or explicitly exempted with a stated reason. Verified it bites: deleting the Worlds clamp line fails it with a message naming the fix.
    3. Wrote the cited test, through acl's exported behaviour rather than the unexported helper.
    4. Error message rewritten to state the grammar directly; the comment notes the grammar is reused but the vocabulary deliberately is not.
    5. Comment now says the check is not implemented and names where it lands, instead of a rule ID that does not exist.
status: addressed
---
