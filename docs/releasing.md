# Releasing

Releases are produced entirely by the `Release` workflow
(`.github/workflows/release.yml`) when a `v*` tag is pushed. GoReleaser
builds the binaries (default + `postgres` variants, all OSes) and
**creates the GitHub release itself** from the tag, then the `desktop`
job attaches the installers.

## Do not pre-create the GitHub release

The empty `v0.8` and `v0.12` releases happened because a release object
was created in the GitHub UI *before* the tag was pushed. When the
build later failed, GoReleaser had no release to clean up (it only
removes a release it created), so a published-looking release with **zero
assets** was left behind — a silent failure with no signal to consumers.

Rule: **never create the GitHub release by hand.** Only push a tag.
GoReleaser owns the release object. If a release run fails, fix the
cause and re-run; GoReleaser's `--clean` will recreate cleanly.

## Cutting a release

Two equivalent options:

1. **Recommended — automated tag bump.** Run the
   [`Tag Release`](../.github/workflows/tag-release.yml) workflow from the
   Actions tab (`workflow_dispatch`). It computes the next `v<minor>` tag
   from the latest existing tag, pushes it from `develop`, and the
   `Release` workflow takes over. This removes the manual step where the
   empty-release footgun lives.

2. **Manual tag push.** From an up-to-date `develop`:

   ```bash
   git tag v0.13 && git push origin v0.13
   ```

   Use the next integer minor — tags are `v0.1`, `v0.2`, … `v0.12`.

## Why a release can silently produce no assets

- **Platform-specific compile break.** A unix-only symbol (e.g.
  `syscall.O_NOFOLLOW`) used without a build-tagged fallback compiles on
  the linux CI runner but breaks GoReleaser's `windows`/`darwin`
  cross-compile. The `cross-compile` CI job
  (`.github/workflows/ci.yml`) now builds every `(GOOS, build-tag)`
  combination GoReleaser ships, so this fails at PR time instead.
- **Gating job failure.** `release` needs `test` and `security`
  (govulncheck). If either fails, `release` is skipped and no assets are
  produced — this is correct gating, not a bug. Bump deps / fix the vuln
  and re-tag.

## Verifying a release

After the workflow finishes, every release must contain both the default
and `postgres` archives per OS/arch, e.g.:

```bash
gh release view v0.13 --json assets --jq '[.assets[].name]'
```

Expected to include `rela_<ver>_<os>_<arch>.tar.gz` **and**
`rela-postgres_<ver>_<os>_<arch>.tar.gz` (`.zip` on Windows), plus the
desktop installers and `checksums.txt`.
