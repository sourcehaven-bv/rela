---
id: DOCS-DGBBGQ
type: docs-checklist
title: 'Docs: drop unused rela-linux binary artifact (TKT-EWNORS)'
status: done
---

## Code Documentation

- [x] Inline comment on the `build` job in `.github/workflows/ci.yml` records that
it is a compile-only gate, that release binaries come from GoReleaser in
`release.yml`, and why the artifact upload was removed

## Project Documentation

- [x] ~~CLAUDE.md~~ (N/A: no architectural rule or convention changes)

## External / User-Facing Documentation

- [x] ~~User-facing docs~~ (N/A: CI-internal change with no effect on the CLI,
the API, or published release artifacts — releases are unaffected)
