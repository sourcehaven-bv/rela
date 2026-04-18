---
id: TKT-AWX7V
type: ticket
title: Upgrade Go toolchain and CI tool versions
kind: chore
priority: medium
effort: s
status: in-progress
---

## Description

Upgrade the Go toolchain and related build/lint tooling both locally (via
Homebrew) and in CI workflows.

**Motivation:**

- Local Go (1.26.2) is ahead of CI's `go-version: '1.25'` — CI is building against an older compiler than development.
- CI pins `golangci-lint@v1.64.8`, but upstream is at v2.x. v1.x is in security-only maintenance and will stop receiving fixes. Major version jump requires a config migration.
- Other brew-managed tools (`just`, etc.) may have pending upgrades that affect the dev loop.
- Repo may have other outdated Go libs or GitHub Actions worth bumping in the same sweep.

**In scope:**

- `brew upgrade` for relevant dev tools (go, golangci-lint, just, goreleaser, wails, etc.)
- Bump `go-version` in all `.github/workflows/*.yml` from `1.25` to `1.26`
- Bump golangci-lint in CI from `v1.64.8` to latest `v2.x`, including any required `.golangci.yml` migration
- Audit and bump other outdated tools/libs (go.mod direct deps, GitHub Actions versions, node tooling if pinned)
- Ensure `just ci` passes locally after upgrades

**Out of scope:**

- Bumping Go module `go` directive / `toolchain` directive (separate concern, may be pulled in if required)
- Dependabot-driven dependency updates for indirect Go deps
- Frontend framework version bumps (Vue, Vite) unless required by tooling
