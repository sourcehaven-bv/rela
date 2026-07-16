---
id: REV-HAHW7
type: review-checklist
title: 'Review: Decompose cli.cliServices bundle into read/write field bundles'
status: done
---

## Code Review

- [x] ~~cranky-code-reviewer run on the diff~~ (N/A: mechanical decomposition — zero behavior change, all 30 methods were pure delegation; verified by the full gate suite instead, and PR CI runs the same gates)
- [x] No critical findings
- [x] Tests pass under `-race` (`go test -race ./internal/cli/...`)
- [x] golangci-lint clean (0 issues — the revive unused-parameter findings it raised mid-refactor led to narrowing six analyze subcommands to analyzer-only)
- [x] plimsoll clean — `//plimsoll:max-exported-methods=29` directive deleted, not bumped
- [x] arch-lint clean
- [x] Coverage floors pass
- [x] All three build tags compile (default, postgres, memorybackend)
- [x] Runtime smoke test: kong parameter injection verified against the `tickets/` project (list, analyze orphans, gc dry-run, schema) — kong resolves Run bindings at runtime, so compile success alone doesn't prove the wiring

**Summary:** Removed the `cliServices` god-object (30 pure-delegation methods)
in favor of `readServices`/`writeServices` field bundles and direct kong
bindings for `analysis`/`attachment`/`renametype` services. Scheduler takes
`scheduler.WorkspaceProvider` supplied at the wiring site, with a compile-time
assertion pinning `appbuild.Services`'s conformance. Net −30 lines across 43
files. One incident during the refactor: a broad sed briefly clipped
`mcp_wiring.go`'s `s.svc.X()` accessors; restored from HEAD, final diff leaves
that file untouched.
