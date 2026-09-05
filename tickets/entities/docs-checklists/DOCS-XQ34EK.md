---
id: DOCS-XQ34EK
type: docs-checklist
title: 'Docs: Why rela import bypasses transition guards'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

The godoc on `importEntity` IS the deliverable. It records three things a reader
cannot infer from the code:

- **Why the guards are absent.** Three reasons weighed together, because each
alone is weaker than the set: import loads states that already exist; the
importer is CLI-only; a guard is a speed bump against someone who already has
store access.
- **Why this diverges from sync**, which enforces (RR-NB135). Sync is an ongoing
channel carrying a peer's new transitions; import is a one-shot operator load of
historical fact. An undocumented divergence between two write paths is exactly
what invites the next review finding.
- **What would change the answer** — a non-CLI caller. Naming the invalidating
condition is what stops the decision being silently inherited into a context
where its third reason no longer holds.

It also states plainly what the argument does NOT claim: that unguarded import
is harmless in general. It claims the guard defends nothing against the only
actor who can reach this code. That distinction is the part most easily
over-generalised by a later reader, so it is spelled out rather than implied.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: no new pattern — it records
why an existing exception exists)
- [x] ~~docs/ updated for changed behaviour~~ (N/A: see Rationale)
- [x] ~~Architecture docs updated~~ (N/A: no boundary, dependency or wiring
change)

## External Documentation

- [x] ~~README updated~~ (N/A)
- [x] ~~CLI reference updated~~ (N/A: `rela import` behaves exactly as before,
and its flags are unchanged)
- [x] ~~API docs updated~~ (N/A: `internal/importer` is not a public API and has
no HTTP surface)

## Rationale for N/A

`rela import`'s user documentation describes WHAT the command does, not which
internal write path it takes. An operator running an import cares that their
data lands; the fact that it bypasses entitymanager is an implementation detail
— right up until someone asks "why doesn't this enforce transitions?", which is
a maintainer or security-reviewer question, not a user one.

That is why all the documentation effort went into the godoc. The audience for
this decision is whoever next reads `importEntity` or next audits the write
paths, and both of them are already in the file.

Deliberately NOT documented user-facing: that import can set arbitrary status.
Writing it in the CLI reference would read as a feature to rely on rather than a
consequence of how migration data works, and it would make the behaviour harder
to tighten later if a non-CLI caller ever appears.

Worth recording for whoever revisits this: the ticket exists because the code
was SILENT, not because it was wrong. The behaviour was already correct; nothing
explained it, so an external review reasonably read the absence as an oversight.
That shape recurs across several findings in this round — a settled decision
that does not read as settled invites the same question repeatedly.
