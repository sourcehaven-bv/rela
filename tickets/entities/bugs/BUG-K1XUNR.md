---
id: BUG-K1XUNR
type: bug
title: 'Release fails: syscall.O_NOFOLLOW undefined on Windows aborts goreleaser cross-compile'
description: 'internal/audit/filesystem.go opened the audit file with syscall.O_NOFOLLOW, which is unix-only. The default CI build runs on linux, so PR CI never caught it; GoReleaser cross-compiles for windows/darwin at release time, where the windows target failed (undefined: syscall.O_NOFOLLOW). One failed target aborts the entire release, so no archives (default OR rela-postgres) were ever published. v0.12 shipped with zero assets and no release ever contained the rela-postgres archives.'
priority: high
why1: 'internal/audit/filesystem.go:146 used syscall.O_NOFOLLOW unconditionally; the symbol is undefined on Windows, so the windows cross-compile fails to build.'
why2: GoReleaser builds every (GOOS, build-tag) target; a single failing target aborts the whole release run, so it published no archives at all (default and rela-postgres alike) and v0.12 ended up with zero assets.
why3: PR CI only compiled on the linux runner — no cross-compile for windows/darwin — so the platform-specific break passed every check and only surfaced at release time.
why4: 'O_NOFOLLOW was added as a security control (refuse to open the audit file through an attacker-planted symlink) without a build-tagged fallback for platforms that lack the flag.'
why5: 'The release pipeline had no PR-time gate mirroring GoReleaser''s target matrix, so any unix-only symbol could reach release; failures were also silent because releases were hand-created in the UI before the tag, leaving published-looking releases with no assets.'
prevention: 'O_NOFOLLOW split into nofollow_unix.go / nofollow_windows.go behind an oNoFollow const (0 on Windows). New CI cross-compile job builds every (GOOS, build-tag) combo GoReleaser ships, gated into the build job, so this class of break fails at PR time. docs/releasing.md documents tag-push-only flow and tag-release.yml auto-bumps the next tag instead of hand-creating the release object (the source of the empty v0.8/v0.12 releases).'
status: done
---

## Bug

`internal/audit/filesystem.go:146` opened the audit log with
`os.O_APPEND|os.O_CREATE|os.O_WRONLY|syscall.O_NOFOLLOW`. `syscall.O_NOFOLLOW`
exists on linux and darwin but is **undefined on Windows**.

The default CI build runs only on the linux runner, so this compiled cleanly
through every PR check. GoReleaser, however, cross-compiles for
`windows`/`darwin` at release time. The windows target failed with:

```text
internal/audit/filesystem.go:146:47: undefined: syscall.O_NOFOLLOW
```

A single failing build target **aborts the entire GoReleaser run**, so the
release published *no* archives — neither the default `rela_*` nor the
`rela-postgres_*` archives. This is why:

- **v0.12** shipped with **zero assets**.
- **No release ever contained the `rela-postgres_<ver>_<os>_<arch>` archives**,
  even though `.goreleaser.yaml` defined them correctly.

(Separately, **v0.8** had zero assets for a different, historical reason: the
`security`/govulncheck job failed on stale-toolchain stdlib vulns, so the
`release` job was correctly skipped. Both empty releases also *looked*
published because the GitHub release object was hand-created in the UI before
the tag was pushed; GoReleaser only cleans up a release it created.)

## Fix

- Split `O_NOFOLLOW` into `internal/audit/nofollow_unix.go`
  (`syscall.O_NOFOLLOW`) and `internal/audit/nofollow_windows.go` (`0`),
  referenced via the `oNoFollow` const. The directory symlink defense in
  `ensureDirSafe` still applies on all platforms; only the per-file open-time
  symlink refusal is unavailable on Windows.
- New `cross-compile` CI job builds every `(GOOS, build-tag)` combination
  GoReleaser ships (3 OS × 2 build-tags), gated into the `build` job — so this
  class of break fails at PR time, not silently at release time.
- `docs/releasing.md` documents tag-push-only flow; `tag-release.yml`
  auto-computes and pushes the next `v0.N` tag instead of hand-creating the
  release object.

## Tests / Verification

- All 24 release build combos (3 OS × 2 arch × 2 build-tags) compile.
- `go test -race ./internal/audit/` passes; golangci-lint + arch-lint clean.
- Empty `v0.8` / `v0.12` releases deleted (tags preserved).
