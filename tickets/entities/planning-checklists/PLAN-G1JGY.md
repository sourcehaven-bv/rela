---
id: PLAN-G1JGY
type: planning-checklist
title: 'Planning: Replace backend per-file coverage ratchet with package floors; add govulncheck + gosec CI gates'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined
- [x] Acceptance criteria documented

**Problem:** The Go backend's per-file coverage ratchet (`.coverage-baseline` +
`baseline-guard` CI job + `post-merge-sync.yml` baseline regeneration) creates
busy-work for trivial refactors because Go's `go cover` tool has no per-line
opt-out. Research across ~25 major Go projects found zero using per-file
ratchets.

**Scope:**

*In scope:*
- Delete backend `.coverage-baseline`
- Remove `baseline-guard` job from `.github/workflows/ci.yml`
- Remove backend baseline regen steps from `.github/workflows/post-merge-sync.yml`
- Rewrite `.testcoverage.yml`: drop `diff:` block; set honest floor thresholds based on
measured actual coverage (see Approach)
- Move `govulncheck` from `security.yml` (weekly-only) to also run in `ci.yml` as a blocking
job on PRs
- Keep weekly `security.yml` run but add auto-file-issue-on-failure so a new CVE with no
concurrent commits still surfaces
- Remove Codecov upload (confirmed not used; no dashboards relied on)
- Add `just govulncheck` recipe
- Update CLAUDE.md

*Out of scope:*
- Frontend `.coverage-baseline` (100% ratchet on small Vue codebase works fine; stays)
- `gosec` enablement (already on in `.golangci.yml:35`)
- `.golangci.yml` changes (done in TKT-AHUNF)
- Auto-PR-to-fix-vulns (separate enhancement; would open `go get -u` PRs from the weekly run)
- Expanding fuzz targets (separate ticket)

**Acceptance Criteria:**

1. `just coverage-check` passes on develop, fails when a package drops below its floor.
2. `.coverage-baseline` deleted; no backend references remain.
3. `baseline-guard` job removed from `ci.yml`.
4. `post-merge-sync.yml` only regenerates frontend baseline.
5. `govulncheck` is a required check on every PR.
6. Weekly `security.yml` run opens a GitHub issue on failure (so CVEs with no commit activity
still surface).
7. Codecov upload step deleted from `ci.yml`.
8. `just govulncheck` recipe exists and calls `scripts/govulncheck-filtered.sh`.
9. CLAUDE.md "Test Coverage" section reflects the package-floor model (no ratchet language).

## Research

- [x] Codebase patterns reviewed

**Measured coverage** (from `go-test-coverage --config=.testcoverage.yml` on
current develop):

Total: **71.6%** (12506/17474 statements)

Per-package (via `go tool cover -func` aggregation):

| Package | Actual | Proposed floor |
|---|---|---|
| internal/errors | 100% | 95 (unchanged) |
| internal/output | 98.5% | 90 (unchanged) |
| internal/secrets | 96.7% | — (no current override) |
| internal/ai | 96.0% | — (not currently set; skip) |
| internal/markdown | 93.8% | 85 (unchanged) |
| internal/entity | 93.2% | 85 (unchanged) |
| internal/filter | 90.2% | 85 (unchanged) |
| internal/dataentryconfig | 92.1% | 70 (unchanged) |
| internal/project | 86.3% | 85 (unchanged — tight but honest) |
| internal/metamodel | 79.3% | 65 (unchanged) |
| internal/importer | 82.4% | 65 (unchanged) |
| internal/dataentry | 68.9% | 60 (unchanged) |
| Total | 71.6% | 65 (raise from 45) |

All current overrides are honest vs measurement. The only numbers that change:
- `threshold.total: 45 → 65` (still gives ~6pp headroom — tight enough to catch "someone
dumped a 500-line untested package" without requiring new tests to meet the
gate).

**Existing tooling:**
- `go-test-coverage` already installed in CI; keep it, just drop `diff:` section.
- `govulncheck` already installed; `scripts/govulncheck-filtered.sh` handles OSV suppression.
- Weekly `security.yml` runs `govulncheck` but does not notify on failure (silent fail in the
Actions tab) — this is a gap we'll close.

## Approach

- [x] Technical approach chosen and documented
- [x] Alternatives considered

**Technical Approach:**

1. Rewrite `.testcoverage.yml`: drop `diff:` block; `threshold.total: 45 → 65`; keep overrides.
2. Delete `.coverage-baseline`.
3. `.github/workflows/ci.yml`: remove `baseline-guard` job, remove Codecov upload step, add
`vulncheck` job (copied from `security.yml`), add `vulncheck` to `build.needs`.
4. `.github/workflows/post-merge-sync.yml`: drop Go coverage regen steps; keep frontend.
5. `.github/workflows/security.yml`: add `gh issue create` / `gh issue comment` step that runs
on failure and uses `gh issue list --search` for idempotency.
6. `justfile`: add `govulncheck` recipe.
7. CLAUDE.md: rewrite "Test Coverage" section; add "Security Checks".

**Files to modify:**

- `.testcoverage.yml` — rewrite
- `.coverage-baseline` — delete
- `.github/workflows/ci.yml` — remove baseline-guard, Codecov; add vulncheck
- `.github/workflows/post-merge-sync.yml` — remove Go baseline regen
- `.github/workflows/security.yml` — add issue-on-failure
- `justfile` — add `govulncheck` recipe
- `CLAUDE.md` — rewrite Test Coverage section; add Security Checks
- `internal/store/storetest/fuzz.go` — remove stale `coverage-baseline` `nolint` comment

**Alternatives considered:**

- *Drop `go-test-coverage`, roll own shell.* Rejected: tool works in floor mode without `diff:`.
- *Drop coverage enforcement entirely.* Rejected: package floors catch whole-package gaps for
near-zero maintenance cost.
- *Remove the weekly `security.yml` run.* Rejected: "new CVE drops with no commits" is real.
- *Use external `peter-evans/create-issue` action.* Rejected: `gh issue create` is simpler.

**Dependencies:** None new.

## Security Considerations

- [x] Input sources identified
- [x] Security-sensitive operations identified

CI/config-only changes. Moving `govulncheck` to PR-blocking **strengthens**
security posture. Issue-on-failure closes a real detection gap in the weekly
job. No regression.

## Test Plan

- [x] Test scenarios documented
- [x] Edge cases identified

**Test Scenarios:**

1. `just coverage-check` on develop passes (verified locally: 71.8% vs 65% floor).
2. `just govulncheck` succeeds locally (verified: no actionable vulns).
3. PR from this branch shows no `Coverage Baseline Guard` check, shows a new
`Vulnerability Check` check, no Codecov comment.
4. Weekly `security.yml` failure path: covered by code review (hard to simulate without a
real vuln).
5. `grep coverage-baseline .github/ justfile .testcoverage.yml CLAUDE.md` returns only frontend
or planning-doc references (verified).
6. `grep -i ratchet CLAUDE.md` returns no backend-section matches (verified).

**Edge Cases:**

- `threshold.total: 65` — future-drop below this requires deliberate adjustment; that's wanted.
- Issue-on-failure idempotency: use `gh issue list --search "<title> in:title"` to find open
issue before creating.
- `post-merge-sync` when only frontend baseline changes: the single-file `git diff --quiet`
check handles this.

**Negative Tests:**

- Lower a package floor below its actual coverage temporarily → `just coverage-check` fails
with clear package-floor violation.

## Risk Assessment

- [x] Risks assessed with mitigations
- [x] Effort estimated

**Risks:**

- **R1: Floor values too strict.** *Mitigation:* Measured floors match existing overrides;
total raised from 45 → 65 (still ~6pp below current 71.8%).
- **R2: Issue-on-failure creates duplicates.** *Mitigation:* idempotent via `gh issue list`
search + update-existing pattern.
- **R3: Merge conflicts on in-flight PRs touching `.coverage-baseline`.** *Mitigation:* land
during quiet window; rebase by dropping baseline changes.

**Effort:** s (half a day — mechanical edits + local verification, done).

## Documentation Planning

- [x] User-facing docs identified

**Documentation Impact:**

- [x] CLAUDE.md — rewrote "Test Coverage" section, added "Security Checks" section
- [x] ~~User guide~~ (N/A: internal CI change)
- [x] ~~CLI help text~~ (N/A)
- [x] ~~README.md~~ (N/A)
- [x] ~~API docs~~ (N/A)

Refactor ticket — no docs-checklist required per workflow rules.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small CI-config refactor; user-led design discussion replaced formal review)
- [x] All critical/significant findings addressed in plan (answered inline with user)

**Design Review Findings:** *(none — skipped)*
