---
id: DOCS-CAN9GM
type: docs-checklist
title: Documentation
status: done
---
<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — the create-uses-concrete-id rationale (matching the production create path) is documented on `grantingAttributions` in `access.go`; the everyone-baseline lift, blank-key tolerance, and first-key-wins dedup invariant are documented at their sites in `mapall.go`/`mapprincipal.go`.
- [x] Function/type docs if public API — `Can`, `CanResult`, `MapAll`, `MapAllResult`, `EveryoneType` carry godoc; the CLI command godoc documents the data-entry-transport caveat, create semantics, and exit-code contract.

## Project Documentation

- [x] ~~README updated~~ (N/A: README is generated; no user-facing surface for a new subcommand)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern; reuses the `rela acl` + consumer-side-interface conventions from who-can/map)
- [x] Help text accurate — `rela acl can` and the no-`--principal` `rela acl map` Kong help + godoc describe args, whole-graph output, and the exit-code semantics.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no manual changelog; release notes derive from commits/tickets)
- [x] ~~API docs updated~~ (N/A: CLI-only; a `docs/cli-reference.md` entry for the full `acl` command family is deferred with the drift/verify slices)
