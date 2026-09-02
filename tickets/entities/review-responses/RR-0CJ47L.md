---
id: RR-0CJ47L
type: review-response
title: 'The warning checked one of the three conditions reject actually needs'
finding: |-
    `reject` fires only when THREE things hold: the mode is reject, a JWT gate is
    wired, AND the principal_property lookup is enabled (router.go:451 --
    `jwtVerified && id == "" && d.Policy().PrincipalPropertyLookupEnabled()`).

    The first version of the warning checked gate wiring alone. So a reject policy
    with no user_entity_type / principal_property was equally inert and the warning
    stayed SILENT -- a second silent-inert state, uncovered, in exactly the shape of
    the bug the ticket exists to close.

    My defence was "LoadPolicy refuses that combination". It does, but
    `acl.NewDeclarative` never calls Validate, so any construction path skipping
    LoadPolicy holds a policy in precisely that state -- including this ticket's own
    new test, which builds a Declarative directly.
severity: critical
resolution: |-
    Added `acl.Policy.RejectEffective(jwtWired bool)`: one predicate covering all
    three conditions, documented as the single definition of "reject can actually
    fire". The warning now reads it and names WHICH condition is missing, since
    "reject does nothing" is not actionable without saying what to change.

    The godoc records that NewDeclarative skips Validate, so the lookup condition is
    checked rather than assumed.

    Pinned by `TestUnmatchedReject_WarnsWhenLookupDisabled`, mutation-verified:
    dropping the lookup term from RejectEffective reddens it alone.

    The lesson is the one the ticket was already about: I wrote a check for "the
    condition I had in mind" rather than for the condition the enforcement site
    actually evaluates. A warning that is wrong about the thing it warns about is
    worse than no warning.
status: addressed
---

## Resolution

Added `acl.Policy.RejectEffective(jwtWired bool)`: one predicate covering all
three conditions, documented as the single definition of "reject can actually
fire". The warning now reads it and names WHICH condition is missing, since
"reject does nothing" is not actionable without saying what to change.

The godoc records that NewDeclarative skips Validate, so the lookup condition is
checked rather than assumed.

Pinned by `TestUnmatchedReject_WarnsWhenLookupDisabled`, mutation-verified:
dropping the lookup term from RejectEffective reddens it alone.

The lesson is the one the ticket was already about: I wrote a check for "the
condition I had in mind" rather than for the condition the enforcement site
actually evaluates. A warning that is wrong about the thing it warns about is
worse than no warning.
