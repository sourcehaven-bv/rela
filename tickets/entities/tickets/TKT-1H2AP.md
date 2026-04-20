---
id: TKT-1H2AP
type: ticket
title: Replace backend per-file coverage ratchet with package floors; add govulncheck + gosec CI gates
kind: refactor
priority: medium
effort: s
status: done
---

## Problem

The Go backend uses a per-file coverage ratchet (`.coverage-baseline` +
`baseline-guard` CI job) borrowed from frontend tooling. For Go this creates
busy-work without commensurate value:

- Go's `go cover` tool offers no per-line opt-out (unlike istanbul's `/* istanbul ignore */`);
only `go-test-coverage`'s function-level `coverage-ignore` comment exists.
- Small refactors or edits to files containing inherently-untestable code (main funcs, syscall
wrappers) trip the ratchet on noise rather than real regressions.
- The workflow maintaining the baseline (`post-merge-sync.yml` regenerates and opens a PR) adds
an extra merge per week and routinely produces conflicts.

Research into ~25 major Go projects (etcd, moby, vitess, k8s, terraform, consul,
vault, caddy, hugo, traefik, containerd, teleport, otel-collector, etc.) found
zero using a per-file ratchet. The per-file ratchet is a Java/Jacoco idiom that
doesn't match Go culture.

## Solution

Drop the per-file ratchet. Keep `go-test-coverage` but use it for package floor
thresholds only. Add `govulncheck` with a path-filtered PR gate plus a weekly
auto-update workflow that opens an auto-merge PR when a fix exists upstream and
falls back to filing a GitHub issue when no fix is available.

## In scope

- Delete `.coverage-baseline`; remove `baseline-guard` job from `ci.yml`
- Remove backend baseline regeneration from `post-merge-sync.yml`
- Rewrite `.testcoverage.yml`: drop `diff:` block; set honest per-package floors
- Add path-filtered `vulncheck` job to `ci.yml` (only runs on PRs touching `go.mod`/`go.sum`)
- Weekly `security.yml`: auto-update PR flow (App token + `gh pr merge --auto`); fall back to
deduplicated GitHub issue when no upstream fix exists
- Add `scripts/govulncheck-fixable.sh` helper
- Add `just govulncheck` recipe; update CLAUDE.md

## Out of scope

- Frontend `.coverage-baseline` (kept — 100% ratchet is effective there)
- `.golangci.yml` changes (covered by TKT-AHUNF)
- `gosec` enablement (already in `.golangci.yml:35`)
- Dedicated floors for `internal/store/fsstore` and `internal/store/memstore` (RR-YLD0H,
deferred)
