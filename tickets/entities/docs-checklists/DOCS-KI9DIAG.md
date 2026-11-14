---
id: DOCS-KI9DIAG
type: docs-checklist
title: 'Docs: TKT-UHKZKOB'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the ratio uses p50, why a missing Server-Timing header is an error, why an unusable baseline is a note and not a failure, why only a passing run becomes the baseline, why the server log drops the pgstore statement lines
- [x] Function/type docs if public API: package doc in `tools/scalerun/doc.go` and the command doc in `cmd/scalerun/main.go`; comment-lint `doclink` clean on `tools/scalerun`

## Project Documentation

- [x] README updated (if applicable): the generated guide table lists the new guide
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new code convention)
- [x] Help text accurate (if CLI changes): `scalerun` flags and the `just scale-run` recipe comment

## User-facing Documentation

- [x] New guide GUIDE-postgres-scale-run (`docs/postgres-scale-run.md`): the supported postgres scale, what the weekly check measures and when it fails, how to run it locally, how to change a limit or reset the benchmark baseline; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL or server change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API change)
