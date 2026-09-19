---
id: RR-Z7BI5E
type: review-response
title: AC 8 asserts an invariant the create_relation position-4 change breaks
finding: AC 8 states 'Every existing 4-argument and 3-argument call site keeps working unchanged.' For create_relation that cannot hold. Today luaCreateRelation (internal/lua/runtime.go:1883-1901) reads only args 1-3; a 4th argument of any type is silently dropped and the call succeeds. Putting the opts table at position 4 gives that slot meaning, so rela.create_relation(a, 'cites', b, 'some body') goes from silently succeeding to raising (per the non-table rule in RR-NWBAR5). In-tree blast radius is zero — verified no Lua script, test or doc example passes a 4th argument (runtime_test.go:1044, relationgrants_wiring_test.go:119,175, acl_bypass_test.go:58,98, docs:1872, examples/*.lua all use exactly 3). But operator-authored scripts live outside the tree in .rela/scripts/, actions/ and schedules.yaml, which is the whole point of the scripting feature. The plan asserts an invariant its own design breaks rather than declaring the break.
severity: significant
resolution: 'Accepted, taking the reviewer''s option (a). AC 8 no longer claims 4-argument calls are unaffected; it now declares the create_relation arg-4 change as a deliberate break, with the justification that the argument was accepted-and-ignored and never part of the documented signature (GUIDE-lua-scripting.md:351 reads three arguments). Zero in-tree callers pass it, verified across Lua scripts, Go tests and doc examples. A release-note line is required. Option (b), putting the relation opts at position 5 to avoid the break, was rejected: a permanently inconsistent opts position between the two bindings is a worse long-term artifact than a documented break with no known affected caller.'
status: addressed
---

## Finding

AC 8 reads:

> **No call site breaks.** Existing 3- and 4-argument calls behave identically.

For `create_relation` that is not achievable under the proposed design, and the
plan's own Risk table calls the change "a doc-comment correction, not a
behaviour change."

**Today** `luaCreateRelation` (`internal/lua/runtime.go:1883-1901`) reads only
arguments 1-3. A 4th argument of any type is silently dropped and the call
succeeds.

**After** the change, position 4 is the opts table. So:

```lua
rela.create_relation(a, "cites", b, "some body")
```

goes from *silently succeeding* to *raising* (per the non-table rule in
RR-NWBAR5) — or, if that rule is not adopted, to being silently ignored again,
which is worse.

## Blast radius

In-tree: **zero**. Verified that no Lua script, Go test or doc example passes a
4th argument to `create_relation` — `internal/lua/runtime_test.go:1044`,
`internal/appbuild/relationgrants_wiring_test.go:119,175`,
`internal/lua/acl_bypass_test.go:58,98`, `docs/lua-scripting.md:1872` and every
`examples/*.lua` call use exactly three.

Out of tree: unknown and unknowable. Operator-authored scripts live in
`.rela/scripts/`, `actions/` and `schedules.yaml` — user files by design, which
is the entire premise of the scripting feature. A 4-argument call out there
breaks.

## Why it matters

The defect is not the break. The break is small, in-tree-clean, and defensible:
the dropped argument was never documented to users (both
`docs/lua-scripting.md:345` and its source
`docs-project/entities/guides/GUIDE-lua-scripting.md:351` read
`rela.create_relation(from, type, to)`), so nobody could have relied on it from
the documentation.

The defect is that the plan **asserts the invariant rather than declaring the
break**. An AC that says "nothing breaks" against a design that breaks something
is how the break ships unannounced and surfaces as a support question.

## Fix

Pick one and write it down:

**(a) Restate AC 8 honestly.** "A 4th argument to `create_relation` was
previously accepted and ignored; it now must be a table. This is a deliberate,
documented break, justified because the ignored argument was never part of the
documented signature." Add a release-note line.

**(b) Put the relation opts table at position 5** as well, leaving 4 a genuinely
dead slot. Breaks nothing; costs a permanently odd signature and an
inconsistency between the two bindings' opts positions.

I would take (a) — the inconsistency in (b) is a worse long-term artifact than a
documented break with no known affected caller. But it must be *stated*, not
asserted away.
