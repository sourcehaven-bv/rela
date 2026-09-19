---
id: DOCS-POHMRR
type: docs-checklist
title: 'Docs: Faced relation history capture and reads carry the source face'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

The godoc carries the whole explanation here, because every reader of this
change is a developer touching the store contract — there is no operator or
end-user altitude to write for.

`store.RelationHistoryQuery.FromFace` states the rule the rest of the change
follows: the tail is part of the key, not a filter over it, so a query naming no
face reads the default tail *specifically* rather than approximately. It also
records that `FromFace` is ignored when `RecordID` is set, since an explicit
lineage handle is the narrower address — a reader who does not know that would
reasonably think the two could contradict each other.

`ListRelationLifetimes` explains why enumeration is scoped to one tail rather
than listing them all: the response carries no face, so a mixed list would hand
a caller asking about the draft edge an opaque `RecordID` belonging to the
published edge, with nothing in the payload to tell them apart. That is the kind
of reasoning that reads as arbitrary restriction without the why.

`recordIDForKey` in both backends replaces a doc paragraph that argued the
OPPOSITE — it previously said default-tail-only resolution was correct and that
"a face-aware caller resolves its own record id and passes it directly". Leaving
that in place would have been worse than no comment: it would have told the next
reader the new behaviour was a bug.

`RelationVersionInput.RecordID` had gone stale in the same way, saying "0 is
invalid — the caller must supply the row's rel_record_id", which stopped being
true once both backends resolved from the key.

Comments that say what a zero face MEANS were added at each default-tail call
site (purge, the storetest accessor), because passing `entity.Face("")` silently
is how the next person reads it as an oversight and "fixes" it.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: no new pattern. This
applies the existing "the tail is part of a relation's identity" rule, already
recorded on `store.DeleteRelationState` and in the root CLAUDE.md's relation
versioning section, to two surfaces that had not yet honoured it.)
- [x] ~~docs/ updated for changed behaviour~~ (N/A: see rationale below)
- [x] ~~Architecture docs updated~~ (N/A: no boundary or wiring change. The
capability interfaces are unchanged in shape; `RelationHistoryReader` gained a
parameter, not a new seam.)

## External Documentation

- [x] ~~README updated~~ (N/A)
- [x] CLI reference updated — the `from` positional on `relation-history` and
`relation-restore` now reads "Source entity ID, optionally faced as ID@face",
which is where a CLI user meets this. That help text IS the CLI reference; there
is no separate document to update.
- [x] ~~API docs updated~~ (N/A: no wire change. The route accepts an address
it always should have accepted, and `from` in responses now echoes the address
the caller used rather than a bare id — a correction, not a new field.)

## Rationale for N/A

No user-facing behaviour was ADDED; behaviour that was already documented as
existing was made to actually work.

`docs/postgres-backend.md` and the root CLAUDE.md already describe relation
versioning as per-lineage with `from_face` on the version rows. A reader of
either would have expected a faced relation's history to be its own — the
documents were correct and the code was not. Writing a new section announcing
that faced relations now get their own history would imply the old behaviour was
the documented contract, which would be false.

The one genuinely new user-visible affordance is the `ID@face` spelling on two
CLI positionals, and that is documented where a CLI user actually looks: the
flag help, updated above.

Deliberately NOT written: a migration or upgrade note. Nothing stored changes
shape, no existing row is rewritten, and the default-tail behaviour every
current caller relies on is unchanged — a caller that names no face gets exactly
what it got before.
