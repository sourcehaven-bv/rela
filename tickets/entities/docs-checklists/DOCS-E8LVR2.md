---
id: DOCS-E8LVR2
type: docs-checklist
title: 'Docs: fs-to-sqlite migration command'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious — `fsimport` package doc, `normalizeProps` (date rules), `recordAudit` (read-back), rename recheck, `copyEntry` mode capping
- [x] Function/type docs if public API — `fsimport.Run`, `Options`, `Backend`, `Opened`, `Target`, `Report`; `appbuild.OpenSQLiteData`

## Project Documentation

- [x] ~~README updated~~ (N/A: README does not list db subcommands)
- [x] CLAUDE.md updated — fs-to-sqlite import listed as the fifth raw-store exception
- [x] Help text accurate — `rela db import-fs --help` matches the CLI reference

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from PR titles)
- [x] ~~API docs updated~~ (N/A: no HTTP API change)
- [x] GUIDE-sqlite-backend: "Moving a filesystem project to SQLite" (what is copied, what is not, next steps, killed-run leftovers); GUIDE-cli-reference: `rela db import-fs`; `docs/` regenerated with `just docs`

## Verification

- [x] Documented command run end to end on a copy of `tickets/` (5492 entities, 6703 relations, matching the source file counts)
