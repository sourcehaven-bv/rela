---
id: DOCS-2ODAKC
type: docs-checklist
title: 'Docs: ctag watermark cross-collection disclosure'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

`store.TypeWatermark` gains a fifth section, placed AFTER the existing
functional-safety argument. That ordering is load-bearing: "we accept this" only
reads as a decision rather than a shrug once the reader knows the alternative is
silent client staleness.

It separates two questions the original doc conflated by omission. The existing
text argues over-triggering is FUNCTIONALLY safe (a spurious re-sync
self-corrects; a missed change strands a client forever). Whether it is
CONFIDENTIAL is a different question, and one that only arises once a single
type is exposed through several differently-authorized collections — which
graph-driven CalDAV collections do.

It then sizes the disclosure honestly (one bit, no id, no content, no timing
resolution finer than the client's poll interval) and gives the three rejected
alternatives with the GDPR/AVG argument as the pivot: narrowing the watermark
would require deletion records to remember which project a deleted entity
belonged to, so a fact about a deleted subject would survive their deletion.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: no new pattern)
- [x] docs/ updated for changed behaviour — see below
- [x] ~~Architecture docs updated~~ (N/A: no boundary or wiring change)

`docs/caldav.md` gains one `Constraints` bullet at OPERATOR altitude, alongside
that section's existing honest caveats. It states what a user of one collection
can infer about others, sizes it, and — unlike the godoc — ends in an action:
tenants for whom the mere fact of activity is sensitive should get separate
entity **types**, not separate collections over one type.

That advice was verified rather than merely offered as reassurance:
`EntityTypeWatermark` keys on entity type, so distinct types genuinely get
independent counters. Advice that sounds right but does not work is worse than
no advice in a security note.

Edited at `docs-project/entities/guides/GUIDE-caldav.md` and regenerated with
`just docs`.

## External Documentation

- [x] ~~README updated~~ (N/A)
- [x] ~~CLI reference updated~~ (N/A: no command or flag)
- [x] ~~API docs updated~~ (N/A: no surface change — this documents an existing
property of the ctag, it does not alter one)

## Rationale for N/A

Nothing observable changed. The ctag computation, the watermark scope and the
tombstone contents are all untouched; this records why they are what they are.

Two audiences, deliberately split — that split is the substance of this
checklist, not an accident of where the text landed:

- The **godoc** carries the full analysis, because its reader is deciding
whether to narrow the watermark and needs to know why they must not. Without it,
the next person to look at a shared ctag re-derives the finding and files the
same issue.
- The **guide** carries one bullet, because its reader is an operator deciding
how to structure tenants. They do not need the tombstone argument; they need to
know the signal exists and what to do about it.

Deliberately NOT written: a mitigation section in the guide beyond the
separate-types advice. There is no mitigation for the shared-type case, and
inventing one would be worse than the honest statement that this is accepted.
