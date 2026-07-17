---
id: DOCS-6FA7HT
type: docs-checklist
title: Documentation
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — the read-vs-runtime rationale, the group-exclusion-removal rationale (RR-C5Q743), and the merge-by-effective-principal reasoning are documented at their sites in `access.go`, `enumerate.go`, `whocan.go`.
- [x] Function/type docs if public API — `AccessRoutes`, `EveryoneGrants`, `Verb`, and the `aclmap` engine/result types carry godoc; the CLI command godoc documents the data-entry-transport caveat and group-entity reporting.

## Project Documentation

- [x] ~~README updated~~ (N/A: README is generated from doc entities; no user-facing README surface for a new subcommand)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting pattern; follows existing `rela acl` + consumer-side-interface conventions)
- [x] Help text accurate — `rela acl who-can` Kong help + command godoc describe the verbs, output, and the `principal_property` data-entry-transport scope caveat.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: project has no manual changelog; release notes derive from commits/tickets)
- [x] ~~API docs updated~~ (N/A: CLI-only; no HTTP/MCP API surface. A `docs/cli-reference.md` entry belongs with the follow-up `map`/`can` slice when the full command family lands — deferred in [[TKT-9089I6]].)
