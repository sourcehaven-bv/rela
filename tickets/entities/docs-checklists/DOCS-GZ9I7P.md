---
id: DOCS-GZ9I7P
type: docs-checklist
title: 'Docs: Actions: gate entity_id on the read path'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] The resolution site in `handleV1Action` explains *why*
`visibility.ScriptReader` is the seam — enumerating the three properties it
provides (stored-type gating, field redaction, oracle-free denial), because a
future reader who only needs one of them would otherwise be tempted to hand-roll
a narrower check, which is exactly the mistake the first attempt made.
- [x] The fall-through-on-deny behaviour is documented as **deliberate**, with
the reason (`entity_id` is an optional parameter, not the resource) and the
concrete consequence of getting it wrong (a bulk list action failing because one
selected row was invisible).
- [x] `v1ActionRequest.EntityType` documents that it is accepted-and-ignored,
why it stays on the wire (the SPA sends it), why the server must never authorize
against it, and — explicitly — *do not "notice it's unused" and wire it back
in*.

## Project Documentation

- [x] ~~`docs/data-entry.md` / guides~~ (N/A: no user-facing behaviour change.
An authorized caller sees exactly what they saw before; only an unauthorized one
is affected, and correctly. Nothing for an operator to configure or learn.)
- [x] ~~`internal/dataentry/CLAUDE.md`~~ (N/A: the governing rule already
exists in the root CLAUDE.md — "Read-out paths go through visibility wrappers,
base readers stay ungated." This bug was a *violation* of that rule, not a gap
in it. Adding a restatement would imply the rule was unclear; it was not, I just
didn't apply it.)

## External Documentation

- [x] ~~Changelog / release notes~~ (N/A: none maintained in-tree; the commit
messages carry the rationale)

## Note

The first commit's message asserted a security property the code did not have
(that `visible:`-hidden fields no longer reached script scope). That is
corrected in the replacement commit, which states plainly that the earlier claim
was false. Leaving a wrong claim in git history unmarked is its own
documentation defect — someone greps for the fix, reads the claim, and stops
looking.
