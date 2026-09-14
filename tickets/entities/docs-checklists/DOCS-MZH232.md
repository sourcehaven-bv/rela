---
id: DOCS-MZH232
type: docs-checklist
title: 'Docs: Scope the timing claim to entity-level filtering'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

No code changed, so no godoc. Worth noting the existing comments were already
adequate here: `internal/dataentry/visiblereader.go` states plainly why gating
is structural rather than by convention, and that comment is what made the
"strength, not shortcoming" framing verifiable rather than assertable.

The defect was in the user-facing guide, not the code.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: no new pattern)
- [x] docs/ updated for changed behaviour — see below
- [x] ~~Architecture docs updated~~ (N/A: no boundary or wiring change)

`docs/acl-security.md`, in the search-visibility section, gains a scoping of an
existing claim. The original sentence — that the postgres build composes
visibility into the search SQL so there is "no hidden-row work to measure
through timing" — is KEPT, because it is true about rows. It is now marked as
being about rows, and followed by what that leaves out.

Edited at `docs-project/entities/guides/GUIDE-acl-security.md` and regenerated
with `just docs`.

## External Documentation

- [x] ~~README updated~~ (N/A)
- [x] ~~CLI reference updated~~ (N/A: no command or flag)
- [x] ~~API docs updated~~ (N/A: no surface change)

## Rationale for N/A

Nothing observable changed — this corrects a claim, not a behaviour.

The judgement worth recording is about TONE, since that is what a documentation
ticket about a security limit gets wrong most easily.

The correction had to avoid two failures, and they pull in opposite directions.
**Overclaiming** was the original state: a reader who trusts "no timing signal"
may build on that assumption, and the correction is cheap now and expensive
after someone has. **Underclaiming** would be the natural over-correction:
describing a microsecond-scale signal as a vulnerability would misdirect effort
and misrepresent a system that does more field-level enforcement than most
applications attempt.

So the wording is deliberate. "Not constant-time" is precise and checkable.
"Vulnerable to timing attacks" would not be — it implies an exploit path that
requires an attacker who already holds a valid principal, already knows which
field to probe, and can distinguish microseconds across a network.

The passage also states why the residual signal EXISTS: because rela does
field-level redaction at all. Without that, a reader meets a limit with no sense
of what it is a limit on, and the next review files the same finding. With it,
the limit reads as the edge of something real rather than as a gap.

Deliberately NOT added: a mitigation section. There is no mitigation, and
inventing advice ("avoid probing timing") would be worse than the honest
statement that this is an accepted trade.
