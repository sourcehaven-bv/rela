---
id: BUG-Y5UF3K
type: bug
title: Gaining a role can narrow field visibility and writability
description: 'A role without a visible:/fields:/options: block adds no fields to the union, so a conferred role with a block narrows what a global role grants on that row.'
priority: high
why1: 'FieldVerdicts skips a role that declares no visible:/fields:/options: block for the type, so that role adds nothing to the union. Once another role on the row declares a block, every field outside that block''s list is denied, including fields the first role could see or write.'
why2: The dimension treats a missing block as no opinion instead of as a grant of every field. The closed-world switch was designed as one per-principal flag (TKT-9E57 DR-S3/DR-C4), which is correct only when every role on the row declares a block.
why3: 'The affordance resolver never consults the verb lists (read/update). Without them it cannot tell an admin holding update on every type and no fields: block apart from a viewer with no update at all, so the only safe-looking default for a block-less role was to add nothing.'
why4: No test pinned monotonicity. Tests covered the closed world of one role and the union of roles that all declare blocks, never a block-less role combined with a conferred role that declares one.
why5: The union rule is stated for verbs (declarative.go, DEC-RG878) but not as an invariant over every dimension. Each dimension (visible, fields, options, the client ceiling) implemented its own fold and opt-in rule.
prevention: 'MEAS-ROLE-UNION-MONOTONIC: TestRoleUnion_Monotonic checks every subset of a role pool and asserts that adding a role never hides a field, makes a field read-only, or denies an option, live, on create and in history. aclaudit A14 flags an everyone role that opens other roles'' blocks. The ACL guides state the union rule for every field dimension.'
started: "2026-10-13"
completed: "2026-10-14"
status: done
---

## Description

`FieldVerdicts` (internal/affordances/resolver.go) folds the roles that apply to
a row into a `dimension` (dimension.go). Only a role that declares a block for
the type opts in and adds fields to `allowed`; a role without a block is
skipped. Once any role opts in, every declared field outside `allowed` is
denied. So an admin (no `visible:` block) who is also linked as `watcher` on a
row loses every field `watcher` does not list on that row. Gaining a role
narrows what the principal sees. The same holds for `fields:` (writable) and
`options:`.

This contradicts the union semantics `declarative.go` states for verbs, and it
makes BUG-WJSYQT refuse admins filter and sort on fields a conferrable role
hides (RR-FFXKEK).

## Fix

Model each role grant per dimension as a set and take the union:
- a role with no block for the type contributes the full declared field set (or every option), but only if the role holds the matching verb on the type (read for `visible:`, update for `fields:`/`options:`);
- a role with a block contributes the fields it lists (a `when:` grant only on rows where it holds).

Check the history path (`IsHistoricalSubject` opts in up front with globals
only) and pin its behaviour with a test. Pin monotonicity with a test: adding
any role to a principal never removes a visible or writable field.
