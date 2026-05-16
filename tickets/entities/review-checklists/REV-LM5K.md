---
id: REV-LM5K
type: review-checklist
title: 'Review: Narrow lua.WriteDeps.EntityManager and autocascade.Mutator to the 5 methods lua actually calls'
status: done
---

## Code Review

- [x] cranky-code-reviewer run on the diff
- [x] No critical findings
- [x] 2 significant findings addressed (symmetry compile-time assertion + doc fix)
- [x] 1 minor finding addressed (breadcrumb in autocascade.Mutator)
- [x] 1 nit won't-fix (em-dash spacing matches surrounding style)
- [x] 1 leverage acknowledged (deduplicate into entity.WriteAPI — not now)
- [x] Tests pass under `-race`
- [x] `just ci` green
- [x] Drift verified manually: a fake method on either Mutator surface is caught immediately at the script boundary

## Disposition

See IMPL-H61L for the full table and drift-test transcripts.

**Headline outcomes:**

- **Net delta is small** (~46 +/-40 across 5 files) but the architectural win is real: `internal/lua` no longer transitively depends on `internal/entitymanager` at all.
- **Symmetry pinned at compile time.** The cross-cast at the `script.LuaScriptRunner.Run` assignment requires `autocascade.Mutator` and `lua.Mutator` to be structurally identical. That was a load-bearing invariant nothing in the test suite checked. Now a single `var _ lua.Mutator = autocascade.Mutator(nil)` declaration catches drift in either direction at the script boundary.
- **CLAUDE.md "interfaces at the call site"** verified: both `Mutator` types live where they're consumed, neither is a copy of the other being maintained for convenience.
