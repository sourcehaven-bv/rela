---
id: DOCS-UIR41P
type: docs-checklist
title: 'Docs: Remote MCP part 1 — go-sdk migration + ACL-gated read seam'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on the new/changed surface — `mcp.Deps`, `mcp.GraphReader` / `GraphCounter`, `appbuild.GatedReads` / `GatedReadBundle` / `GatedGraphReader`
- [x] Non-obvious decisions explained where they live — why `Deps.Store` is the narrow reader rather than `store.Store` (makes an ungated read unavailable, not merely discouraged); why counts stay ungated while rows are gated; the documented `resources/list_changed` behaviour change from the SDK swap
- [x] Comments explain WHY, not WHAT

## Project Documentation

- [x] ~~New user-facing guide~~ (N/A: part 1 is an internal migration plus a wiring seam; `rela mcp` behaviour is unchanged, so there was nothing for a user to do differently)
- [x] Architecture notes captured — the read seam is described at `internal/cli/mcp_wiring.go`, including why stdio deliberately keeps `acl.NopACL`
- [x] Ticket records the outcome — TKT-UIR41P carries the scope note explaining the split

## External Documentation

- [x] ~~Changelog entry~~ (N/A: generated from commits)
- [x] ~~Migration notes~~ (N/A: no operator-visible change; the user-facing remote endpoint arrives in TKT-BDG8U9, documented there)
