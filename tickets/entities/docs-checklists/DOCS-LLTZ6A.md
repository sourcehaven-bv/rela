---
id: DOCS-LLTZ6A
type: docs-checklist
title: 'Docs: Skip scheduled mail when no section has content visible to the recipient'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols have doc comments
- [x] Non-obvious decisions explained with rationale
- [x] ~~Package doc updated~~ (N/A: no new package, no change to `mailtemplate`'s purpose)

`Template.RequireVisibleContent` documents that the decision uses the
contributed count rather than the matched count, and points at `Build`.

`Build`'s doc carries the load-bearing distinction: the returned count is NOT
the match count, `detail` with an empty body is the case that matters, and
emptiness is decided in the builder (not by inspecting the rendered message)
because each style stores content in a different field.

`skipEmptyContent` records the two decisions a future reader would otherwise
re-litigate: Info rather than Warn (suppression is configured intent, not a
defect) and `nil` rather than an error (child jobs are `RetryBounded`, so an
error would re-render forever and never succeed).

`appendEntity` states that only `detail` can decline — the invariant that keeps
`list`/`table` from acquiring a spurious content guard.

## Project Documentation

- [x] `docs/mail.md` updated — new "Skipping recipients with nothing to read"
- [x] ~~`docs/metamodel.md`~~ (N/A: not a metamodel feature)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no command added or changed)
- [x] ~~`docs/data-entry.md`~~ (N/A: no UI surface)
- [x] ~~`CLAUDE.md`~~ (N/A: no new cross-cutting pattern; existing mail rules unchanged)
- [x] ~~`README.md`~~ (N/A: not a project-level change)

The docs section covers: the YAML shape, that suppression triggers when no
section received content (from either non-matching or non-visible entities), the
contributed-vs-matched distinction and its effect on `{{count}}`, the
default-off guarantee, the new load-time rejection of the sections-less
combination (RR-RV093C), and the `INFO` log line.

It also states plainly what this is **not**: a routing convenience rather than
an access control. If every recipient can read a section's entities, every
recipient still gets mail. That warning exists because the feature is easy to
mistake for a restriction, and the ticket's originating use case depends on
visibility already being scoped.

## External Documentation

- [x] ~~API docs~~ (N/A: no HTTP surface; scheduler-internal)
- [x] ~~Migration notes~~ (N/A: purely additive, default-off, no config migration)
- [x] ~~Changelog~~ (N/A: repo keeps no CHANGELOG; git history carries the ticket ID)

## Examples

- [x] Doc example is runnable as written
- [x] Example matches the originating use case

The `docs/mail.md` example is the ticket's own `mt_agenda` shape. The identical
config was executed end-to-end during implementation verification (recorded in
IMPL-JHJSN9), and the malformed variant was run through the real `rela validate`
to confirm the error message an operator would actually see.
