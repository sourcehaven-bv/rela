---
id: DOCS-9X2BAR
type: docs-checklist
title: 'Docs: Add comments.Store.Get so a single-comment read stops pulling the whole thread'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols have doc comments
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

`Store.Get`'s interface doc states the face-scoping contract and why the method
exists separately from `List`. Each backend's `Get` records what its own
implementation turns on: the two database ones name the index that serves it,
`sqlitecomments` additionally notes that `=` is byte-exact where `LIKE` is not,
and the file/memory ones say why reading the whole thread costs them nothing.

`ErrNotFound`'s doc and the `Store` interface's `Nil:` contract line were both
updated to mention `Get`, rather than left to go stale — the `duplication`
finding commentlint warns about is exactly "this fact is stored in two places
and only one was updated".

## Project Documentation

- [x] ~~docs/ updated for behaviour changes~~ (N/A: no behaviour change)
- [x] ~~CLAUDE.md updated for new patterns~~ (N/A: no new pattern)

No user-visible behaviour changed. The comment API returns the same JSON, the
same routes, the same status codes; `Get` is an internal interface method that
makes an existing operation cheaper.

`CLAUDE.md`'s comments rule was checked and deliberately left alone. It says
"Any new one must pass `internal/comments/commentstest.RunAll`" — still exactly
true, and now covering more. Editing it to enumerate interface methods would
create a second place for the method list to go stale, which is the failure mode
that section's own guidance warns against.

`docs/comments.md` describes the feature from the operator's point of view and
does not document the `Store` interface, so it needed no change.

## External Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~API docs updated~~ (N/A: wire format byte-identical)
- [x] ~~Migration notes~~ (N/A: no schema or data change; no new index)

## Verification

- [x] Documentation matches implementation

The one prose claim this change introduced that was not self-evidently true —
that `=` is byte-exact in SQLite where `LIKE` folds case — is now backed by a
test in `RunKeyFidelityTests` rather than left as an assertion in a comment
(RR-40OP0D).
