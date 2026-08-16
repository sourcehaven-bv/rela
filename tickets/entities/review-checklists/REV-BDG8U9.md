---
id: REV-BDG8U9
type: review-checklist
title: 'Review: Remote MCP part 2 — serve the MCP endpoint over Streamable HTTP'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full `./internal/...` + `./cmd/...` suite green
- [x] Lint clean (`just lint`) — `golangci-lint` 0 issues; `just arch-lint` OK; `just plimsoll` OK
- [x] Coverage maintained (`just coverage-check`) — package floors held

`./scripts/govulncheck-filtered.sh` exits 0 (only GO-2026-4923 ignored, no
upstream fix).

## Code Review

- [x] Run `/code-review` command — externally reviewed on PR #1343 (IB-review, CISO): **approved with findings**
- [x] All critical review-responses addressed — none raised
- [x] All significant review-responses addressed — RR-H8S10M fixed here; RR-P34E8J and RR-PQ5UN1 formally `deferred` with reasons, not closed as done
- [x] Self-reviewed the diff for unrelated changes

The one review finding (Matig) was that **no CI ran on this branch** — correct,
and it is filed and fixed as BUG-CI7XKP with its own 5-whys. My own status
reporting had been wrong: I told the user "ALL GREEN" for a branch that never
built, because I checked for absent failures instead of present jobs.

## Verification

- [x] Acceptance criteria met — 6 of 8; AC 7 and 8 deferred with recorded reasons
- [x] Manual testing performed — see IMPL-BDG8U9's mutation-testing notes
- [x] No regressions introduced — the endpoint is absent unless `-mcp` is set, so an upgraded server is unchanged
- [x] Documentation updated — `docs/mcp-server.md` (remote section, corrected audit semantics) and `docs/server-security.md` (the three known gaps)

## Known gaps carried forward

Recorded on the ticket and in `docs/server-security.md`, not left implicit:
no RFC 9728 discovery, `acl.Request` batch concurrency, and no per-transport
tool allowlist (every stdio tool is remotely reachable, including the sandboxed
Lua ones).
