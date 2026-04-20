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
zero using a per-file ratchet. The dominant norm is either no coverage gate at
all or a loose project-level `target: auto` with 1–15% tolerance, often
informational only. The per-file ratchet is a Java/Jacoco idiom that doesn't
match Go culture.

## Solution

Drop the per-file ratchet. Keep `go-test-coverage` but use it for package floor
thresholds only (coverage must stay above a minimum, but can freely move within
that band). Reinvest the saved review cycles into signals that actually catch
bugs:

- `govulncheck` on every CI run (currently weekly only; should be blocking on PRs)
- `gosec` on the AI/network/Lua paths, which represent the new threat surface per CLAUDE.md

## In scope

- Delete `.coverage-baseline` (backend only; frontend 100% ratchet stays — it works there)
- Remove `baseline-guard` job from `.github/workflows/ci.yml`
- Remove backend baseline regeneration from `.github/workflows/post-merge-sync.yml`
- Rewrite `.testcoverage.yml`: drop `diff:` section, keep/tune package floors
- Move `govulncheck` from `security.yml` (weekly) into `ci.yml` (blocking on PRs)
- Add `just govulncheck` target and document in CLAUDE.md

## Out of scope

- Frontend `.coverage-baseline` (Vue project, 100% ratchet is cheap and effective there)
- Any change to `golangci-lint` configuration (`.golangci.yml` already enables `gosec`,
`errorlint`, `bodyclose`, etc. — this was covered by TKT-AHUNF)
- Fuzz target expansion (separate ticket if we want it)
