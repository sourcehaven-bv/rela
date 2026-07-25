---
id: DOCS-MAP1B7
type: docs-checklist
title: Documentation
status: done
---
<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — the baseline/exception split rationale, the empty-type fix, and the isTypeLevel/assertTypeLevel guards are documented at their sites in `mapprincipal.go`.
- [x] Function/type docs if public API — `MapPrincipal`, `TypeAccess`, `EntityException`, `MapPrincipalResult` carry godoc; the CLI command godoc documents the data-entry-transport caveat.

## Project Documentation

- [x] ~~README updated~~ (N/A: README is generated; no user-facing surface for a new subcommand)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern; reuses the `rela acl` + consumer-side-interface conventions from who-can)
- [x] Help text accurate — `rela acl map` Kong help + godoc describe `--principal`/`--verb`/`--type`, the baseline/exception output, and the cut-off signal.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no manual changelog; release notes derive from commits/tickets)
- [x] ~~API docs updated~~ (N/A: CLI-only; a `docs/cli-reference.md` entry for the full `acl` command family is deferred to the whole-graph `map` slice)
