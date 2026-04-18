---
id: PLAN-DT15Y
type: planning-checklist
title: 'Planning: Upgrade Go toolchain and CI tool versions'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope:**

1. Audit and `brew upgrade` dev tools relevant to this repo: `just`, `markdownlint-cli`, `gopls`. (`go 1.26.2` and `golangci-lint 2.11.4` are already current locally.)
2. Bump CI `go-version` in all `.github/workflows/*.yml` from `1.25` to `1.26`.
3. Bump CI golangci-lint from `v1.64.8` → latest `v2.x` (currently `v2.11.4`). Run `golangci-lint migrate` to auto-convert `.golangci.yml` v1 → v2. Review migrated config, fix any lints the new linter surfaces.
4. Bump GitHub Actions: `codecov/codecov-action@v4` → `@v5` (v6 introduces node24 which may not be safe yet; pick v5 as conservative choice). All other Actions already on latest major (`checkout@v4`, `setup-go@v5`, `setup-node@v4`, `upload-artifact@v4`, `goreleaser-action@v6`, `markdownlint-cli2-action@v19`, `go-test-coverage@v2`, `dependabot/fetch-metadata@v2`).
5. Bump direct Go deps with safe minor/patch updates. Skip major-version changes (e.g., `olekukonko/tablewriter v0.0.5 → v1.1.4` is a major API revision — defer).

**Out of scope:**

- Bumping `go 1.24.0` directive or `toolchain go1.25.8` in `go.mod` — the `go` directive encodes minimum supported Go; the `toolchain` encodes the pinned compiler. These have independent compatibility implications and are tracked separately.
- Upgrading `tablewriter` across its v0→v1 API break.
- Frontend (`vue`, `vite`, `typescript`, `vitest`, `pinia`) version bumps — not part of this sweep.
- Indirect Go dependencies — Dependabot handles those.
- `go.mod` deps whose update requires matching toolchain changes or chained upgrades (we stay within the current `go 1.24.0` / `toolchain go1.25.8` combo).

**Acceptance Criteria:**

1. **Local dev tools current.** `brew outdated` shows no pending upgrades for `just`, `markdownlint-cli`, `gopls`. Verify via `brew list --versions <tool>`.
2. **CI Go version updated.** All `.github/workflows/*.yml` files reference `go-version: '1.26'`. Verify via `grep -r "go-version" .github/workflows`.
3. **golangci-lint v2 in CI.** `ci.yml` installs `golangci-lint@v2.11.4` (or latest v2.x). `.golangci.yml` has `version: "2"` header. `just lint` passes locally against the migrated config.
4. **Actions bumped.** `codecov/codecov-action@v5` in `ci.yml`. All other Actions unchanged (already latest major).
5. **Direct Go deps bumped (safe minors).** `go list -m -u -f '{{if and .Update (not .Indirect)}}...{{end}}' all` shows no pending safe minor updates except known-deferred ones (tablewriter v0→v1).
6. **`just ci` passes locally** after all changes (lint + test + coverage-check + build).

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Prior art TKT-012** ("Fix Go version mismatch in CI"): previous chore ticket addressed an earlier go.mod/CI toolchain misalignment by adding a `toolchain` directive. Same concept-to-feature pattern applies here (`affects: ci-pipeline`, `implements: FEAT-022`).
- **golangci-lint migration tool**: `golangci-lint migrate` is upstream's official CLI for v1→v2 config conversion. Converts `.golangci.yml` in-place. Described at https://golangci-lint.run/product/migration-guide/. Preferred over hand-editing.
- **`brew outdated`** surfaces the local tool upgrade set; filtered on repo-relevant names. Uses existing developer dependencies — no new tooling.
- **`go list -m -u -f '{{if and .Update (not .Indirect)}}…'`** lists direct-deps-with-updates. Standard Go toolchain call; no third-party dep audit tool needed.

**Repo-relevant brew-outdated findings (this session):** `just` (1.46.0 →
newer), `markdownlint-cli`, `node`, `gopls`. `go`, `golangci-lint` already at
latest.

**Direct Go deps with updates:** chromedp, fatih/color, go-git/v5, mcp-go,
mattn/go-runewidth, olekukonko/tablewriter (v0→v1 — major, skip), cobra, pflag,
testify, wails/v2, goldmark, gopher-lua, golang.org/x/net, golang.org/x/sync.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Do the upgrade as one PR in staged commits so a bisect lands on the precise
offender if anything breaks:

1. **Commit 1 — local brew upgrades.** `brew upgrade just markdownlint-cli gopls`. Developer environment only; no repo changes. (This commit may be a no-op in the PR — it's only listed here for the user's local environment.)

2. **Commit 2 — bump CI `go-version`.** `sed`-style replace `go-version: '1.25'` → `go-version: '1.26'` across `.github/workflows/*.yml` (also handles `"1.25"` quoting variant in release.yml/security.yml). Verify with grep.

3. **Commit 3 — golangci-lint v2 migration.**
   - `cp .golangci.yml .golangci.yml.bak` (locally, don't commit the backup).
   - Run `golangci-lint migrate` to produce v2 config.
   - Review the diff. Migrations commonly move `linters-settings` → `linters.settings`, `run.skip-dirs` → `issues.exclusions.paths`, formatters split out of linters. Reference: https://golangci-lint.run/product/migration-guide/.
   - Bump pin in `ci.yml` line 257: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8` → `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4`. Note the v2 module path has `/v2/`.
   - Run `just lint` locally. Address any new findings or explicitly disable in config with a comment explaining why.

4. **Commit 4 — bump `codecov/codecov-action@v4` → `@v5`** in `ci.yml` line 37.

5. **Commit 5 — bump direct Go deps.** For each safe minor/patch in the list: `go get <module>@<version>`, then one `go mod tidy`. Exclude `tablewriter` (v0→v1 API break) — capture as a follow-up ticket if desired. After each batch, run `just test` to catch regressions.

6. **Verification commit (if needed).** If `golangci-lint v2` flags existing code and fixing requires touching more than trivial config, those fixes land as a final commit with "fix: resolve golangci-lint v2 findings".

**Files to modify:**

- `.github/workflows/ci.yml` (go-version ×5 occurrences, golangci-lint install line, codecov-action version)
- `.github/workflows/coverage-ratchet.yml` (go-version)
- `.github/workflows/release.yml` (go-version ×4)
- `.github/workflows/security.yml` (go-version)
- `.golangci.yml` (v1 → v2 migration, possibly new settings/exclusions)
- `go.mod`, `go.sum` (direct-dep bumps)
- No source code changes anticipated unless v2 lints surface real issues

**Alternatives considered:**

- **Hand-write v2 config from scratch.** Rejected: migrate tool handles the mechanical bits, reduces chance of dropping a working rule.
- **Split into multiple PRs (one per concern).** Rejected for this scope — all three concerns (Go version, linter, deps) are coupled by "does CI still pass after brew upgrades", and a single PR gives a cleaner rollback point. Staged commits within one PR preserve bisect-ability.
- **Bump `codecov/codecov-action` to v6.** Rejected for now: v6 switches to node24, which may break runners. v5 is the safe choice.
- **Bump tablewriter to v1.** Rejected: major API change, deserves its own ticket to review call sites.
- **Include frontend bumps.** Rejected by user scope: out of this ticket.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

No new runtime input surface — this is tooling/CI config. Validation is
reviewer-eye on the migrated `.golangci.yml` diff.

**Security-Sensitive Operations:**

- **Go dep bumps** pull in new upstream code. Mitigation: `govulncheck` runs in the `security.yml` workflow and will flag any known-vuln in the new dep graph. Read CI output before merging.
- **golangci-lint v2** enables `gosec` (already enabled in v1 config). New v2 security checks may surface real findings — address them, don't disable the check.
- **GitHub Action bumps** — always a supply-chain risk. We pin to immutable major tags (`@v5`), matching the repo's existing pattern. Not introducing any new unpinned Action.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| # | Acceptance Criterion | How Tested |
|---|----------------------|------------|
| 1 | Local dev tools current | `brew outdated` shows no pending for `just`, `markdownlint-cli`, `gopls` |
| 2 | CI Go version 1.26 | `grep -r "go-version" .github/workflows` shows only `1.26` |
| 3a | `.golangci.yml` v2 | File starts with `version: "2"` |
| 3b | `just lint` passes | Local run returns 0 |
| 4 | codecov-action v5 | `grep codecov .github/workflows/ci.yml` shows `@v5` |
| 5 | No pending direct-dep safe-minors | `go list -m -u -f '{{if and .Update (not .Indirect)}}{{.Path}}{{end}}' all` returns only known-deferred (tablewriter) |
| 6 | `just ci` passes | Local run: lint + test + coverage-check + build all green |

**Integration test** = pushing the PR and watching every job in `ci.yml`,
`coverage-ratchet.yml`, `security.yml` turn green. That's the definitive gate.

**Edge Cases:**

- **`golangci-lint migrate` drops a setting.** Spot-check by diffing enabled linter list before/after; confirm counts match.
- **New linter v2 findings in codebase.** Either fix in code (preferred) or add a targeted exclusion with explanatory comment. Never disable a linter category wholesale.
- **Dep bump breaks build but not tests.** Catch via `just build` / `just build-cli` / `just build-server` / `just build-desktop` — `just ci` includes `build`.
- **Wails v2.11 → v2.12 wants matching Wails CLI.** If `just build-desktop` fails, document in the PR and revert that specific bump.
- **`go 1.26` as CI target with `go 1.24.0` minimum in go.mod.** Supported combo — `go-version: '1.26'` in setup-go installs 1.26 but `go.mod` constraint lets older users still build. No action needed.
- **govulncheck flags a vuln in a bumped dep.** Escalate: either jump to a patched version, or revert that specific bump and file a follow-up ticket.

**Negative Tests:**

- Deliberately break config (e.g. run pre-migration `.golangci.yml` through v2 binary) → expect config error with clear message; confirms v2 is actually in effect.
- `go install …@v2.11.4` against the wrong module path (no `/v2/`) → fails to build. Our fix uses the `/v2/` path so this wouldn't regress, but worth knowing.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| golangci-lint v2 surfaces N new findings, balloon scope | Medium | Medium | Set a budget: if > ~20 findings or any requires a redesign, revert the migrate commit and file a separate TKT to do lint cleanup in isolation. |
| Dep bump introduces a subtle behavioral regression | Low | Medium | Bump in isolated commits and run `just test` between batches; coverage ratchet catches obvious drops. Revert offender on red. |
| `setup-go@v5` with `go-version: '1.26'` needed a not-yet-released go point release | Low | Low | setup-go resolves minor versions; if 1.26.x isn't on runners yet, falls back. Worst case: pin `'1.26.2'` explicitly. |
| codecov v5 changes upload auth/token handling | Low | Low | Read v4→v5 release notes before the bump; verify a PR upload works. |
| Wails v2.12 breaks desktop build | Medium | Low | Desktop build runs in `just ci`. Revert bump if it fails; desktop is optional. |

**Effort estimate:** **s** (small). Most of the work is running tools and
reading their output. The one swing factor is the size of the v2 lint fallout.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [ ] User guide / reference docs
- [ ] CLI help text (if commands changed)
- [ ] CLAUDE.md (if new patterns)
- [ ] README.md (if project-level changes)
- [ ] API docs (if applicable)
- [x] N/A - Internal change, no user-facing docs needed

Tooling/CI bumps are invisible to users. No docs-checklist needed.

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** (pending user approval of plan)
