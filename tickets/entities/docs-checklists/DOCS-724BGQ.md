---
id: DOCS-724BGQ
type: docs-checklist
title: 'Docs: Help endpoint is public by design'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

The godoc on `handleEntityHelp` IS the deliverable. It records what a reader
cannot infer from the code:

- **The open-source test** — the entity model, field descriptions and help prose
all live in the repository, so guarding the endpoint that serves them protects
nothing an interested party cannot read on GitHub.
- **The already-public sibling**, `handleV1Schema`, named so a reader can CHECK
the claim rather than take it on trust.
- **What would change the answer** — if `_schema` were ever read-gated.

It also names the distinction the finding conflated: this endpoint discloses the
entity TYPE model, never an instance, a property value, or anything
ACL-governed.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: no new pattern — it records
why an existing endpoint is ungated)
- [x] ~~docs/ updated for changed behaviour~~ (N/A: see Rationale)
- [x] ~~Architecture docs updated~~ (N/A: no boundary or wiring change)

## External Documentation

- [x] ~~README updated~~ (N/A)
- [x] ~~CLI reference updated~~ (N/A: no command or flag)
- [x] ~~API docs updated~~ (N/A: no behaviour, route or response change)

## Rationale for N/A

The planning checklist left this genuinely open — "whether a user-facing note is
warranted is a real question, since 'the help endpoint is unauthenticated' reads
differently to an operator than to a maintainer". Settled here as NO, for a
reason worth recording rather than defaulting to.

Documenting "help is unauthenticated" in an operator-facing guide would be
actively misleading in both directions. An operator would reasonably read it as
a caveat about THIS endpoint — implying the surrounding surface is authenticated
in a way this one is not — when in fact the schema endpoint beside it is equally
public and the data endpoints are all gated. The sentence would create a
distinction that does not exist.

It would also invite the wrong follow-up. "Should we put it behind auth?" is a
question that only makes sense if help is anomalous, and it is not; it is
consistent with `_schema`. The right place for that reasoning is next to the
code, where someone asking the question is already looking.

Deliberately NOT documented user-facing: what the endpoint exposes. Writing "the
help endpoint reveals your entity type names and field descriptions" reads as a
disclosure warning, which would be a strange thing to say about content the
project publishes on GitHub.

Worth recording for whoever revisits this: the ticket exists because the code
was SILENT, not because it was wrong. That shape recurs across several findings
in this round — a settled decision that does not READ as settled invites the
same question repeatedly, and the fix is a comment rather than a config change.
