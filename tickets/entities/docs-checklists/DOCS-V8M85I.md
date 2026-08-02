---
id: DOCS-V8M85I
type: docs-checklist
title: Documentation
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

The new `internal/imgproc` package has a package doc comment stating the
security thesis, godoc on all exported symbols (`Normalize`, `Config`, `Format`,
sentinel errors, `MemoryBudgetBytes`), and the two riskiest functions
(`decodeBounded`, the EXIF/GIF parsers) carry detailed comments on their
untrusted-input contracts and the timeout/semaphore tradeoff.

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level surface change)
- [x] ~~CLAUDE.md updated~~ (N/A: feature follows existing Processor/transform patterns; no new cross-cutting convention)
- [x] Help text accurate — `rela attach` gains native `image:` transforms transparently (same PolicyProcessor path); no CLI flag/help change needed.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: this repo has no CHANGELOG; user docs are generated from `docs-project/`)
- [x] API docs updated — `docs/attachment-security.md` (via `docs-project/entities/guides/GUIDE-attachment-security.md`) gained a "Native image transform (`image`)" section; `docs/metamodel.md` transform row updated to `{cmd: [...]}` or `{image: {...}}`. Both regenerated via `just docs`.
