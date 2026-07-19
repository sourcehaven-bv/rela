---
id: DOCS-XUX51
type: docs-checklist
title: 'Documentation: Close the =~ ReDoS hole: require trusted literal regex patterns (issue #1139)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported/public symbols documented — the changed surface is internal
(`compareRegex`, `validateRegexLiteral`, the two constants); each carries a
docstring stating what it does *and what it deliberately does not do*.
- [x] Non-obvious decisions explained with rationale — the module doc gained a
**"Threat model — `=~` regex (why patterns must be literals)"** section: which
input is trusted and why, the measured reason length caps cannot bound ReDoS,
the accepted operator foot-gun, and the named upgrade path (RE2 / Worker
timeout) if `=~` ever needs a data-sourced pattern.
- [x] **Corrected misleading pre-existing docs.** Three docstrings claimed the
length caps bound ReDoS — disproven and now false-by-measurement:
  - `MAX_REGEX_LENGTH` — relabelled "sanity bound on config, not a ReDoS control".
  - `validateRegexLiteral` — the "coarse ceiling on ReDoS" claim removed.
  - `compareRegex` — new docstring: the cap is hygiene bounding *linear* work;
what makes it safe is requiring a trusted literal. Leaving these would have been
worse than the original bug: they would teach the next reader the same false
model that produced it.
- [x] Documented an invariant the code silently depended on — the parse `cache`
is unbounded *by design*, safe only because sources are operator config.

## Project Documentation

- [x] ~~docs/data-entry.md~~ (N/A: no user-facing surface to document — the engine
is dormant, zero importers, and `visible_when`/`required_when` appear in no
config yet. Documenting the condition language before its first consumer would
be premature and would drift.)
- [x] ~~docs/metamodel.md / cli-reference.md / README.md~~ (N/A: no metamodel,
CLI, or project-level surface changed.)
- [x] ~~CLAUDE.md~~ (N/A: no new cross-cutting pattern or convention. The rule
here is local to one utility and is documented at the point of use.)

## External Documentation

- [x] ~~Migration notes / changelog~~ (N/A: no consumer exists, so no caller can
break. The `=~` grammar narrows — `form.v =~ form.pat` now throws at parse — but
nothing in-tree uses it. Verified: `grep` finds zero importers of
`utils/conditions` and no `visible_when`/`required_when` in any config.)
- [x] GitHub issue #1139 to be closed by the PR, with the rationale that its
proposed remedy (cap the value) was disproven and what replaced it.

**Note on where this is documented:** deliberately in-code rather than in
`docs/`. The audience for a threat model on a dormant internal utility is the
next implementer (the one who wires up a consumer), and they will be reading
`conditions.ts` — not a docs page that would have drifted by then.
