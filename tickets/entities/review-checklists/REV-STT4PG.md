---
id: REV-STT4PG
type: review-checklist
title: 'Review: Release workflow ships rela-server with an empty embedded SPA (GoReleaser job never builds the frontend)'
status: done
---

## Automated checks

- [x] `go test ./internal/dataentry/...` — pass
- [x] `bash -n` on both scripts — clean
- [x] Workflow YAML parses (`yaml.safe_load`)
- [x] `scripts/check-embedded-spa-test.sh` — 17/17 pass
- [x] Work tree clean after frontend build (`ci-clean-worktree-guard` satisfied)
- [x] ~~`just lint` / `just coverage-check`~~ (N/A: no Go code changed; the diff
is a CI workflow plus two shell scripts)

## Code review

Reviewed by cranky-code-reviewer. Six findings, all verified independently
before acting — two critical, three significant, one minor. All addressed.

| ID | Severity | Finding | Status |
| --- | --- | --- | --- |
| RR-1AKW6R | critical | Only `rela-server` in `rela_*` guarded; postgres archive unreachable by the glob | addressed |
| RR-P82DXY | critical | grep exit 2 masked into "no assets found" | addressed |
| RR-0ZGMYC | significant | tree check passed on a zero-byte entry asset | addressed |
| RR-D0HFUH | significant | `app_editor_dist` same failure mode, unguarded | addressed |
| RR-2ZNZ3R | significant | hardcoded `index-` pattern invites loosening | addressed |
| RR-88CBD0 | minor | guard untested; /tmp reuse; message accuracy | addressed |

Two reviewer claims were checked and one corrected: the review asserted the
`rela` CLI embeds the SPA because it links `internal/dataentry`. It does not —
the linker drops the unused embed (verified: 0/37 assets matched). Asserting on
`rela` would have failed every release, so it is deliberately excluded from the
packaged-binary loop.

## Acceptance verification

| Criterion | Result |
| --- | --- |
| Release job builds SPA before GoReleaser | **PASS** — step order verified by parsing the YAML |
| Packaged binary starts clean, serves non-empty index.html | **PASS** — `GET /` 200 / 3397 B; bundle 200 / 31848 B |
| Release fails when SPA missing/empty | **PASS** — real step body against a staged `dist/` containing the genuine broken v0.14 binary exits rc=1 |

Coverage confirmed empirically — all three asserted binaries match 37/37 entry
assets: `rela-server`, `rela-docs`, `rela-server-postgres` (`-tags postgres`).

## Notes

Guard design changed during review from a hardcoded `assets/index-<hash>`
pattern to deriving expected asset names from the built `index.html`. That
removes the rot risk and is strictly stronger: it proves the artifact embeds
*this* build, so a stale binary carrying a previous build's assets also fails.

Scope split with TKT-O03TB recorded on the `depends-on` relation; the broader
spawn-and-HTTP-GET smoke test and `just smoke` wiring stay there.
