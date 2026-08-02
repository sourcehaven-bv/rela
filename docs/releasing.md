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

## Versioning scheme (CalVer)

Releases are date-based. The tag format is **`vYY.M.BUILD`**:

| Part    | Meaning                                                  |
| ------- | -------------------------------------------------------- |
| `YY`    | Two-digit UTC year                                       |
| `M`     | Month, no leading zero (`1`–`12`)                        |
| `BUILD` | `0` for the month's first release, then `1`, `2`, …      |

```text
v26.7.0        first release of July 2026
v26.7.1        second release that month
v26.7.2-alpha  a prerelease
v26.8.0        first release of August — the counter resets
```

The day is not encoded in the tag. `git log -1 <tag>` and the GitHub release
date carry it, and keeping the counter monthly means it stays a small readable
number instead of an encoded date.

Tags before this scheme were `v0.1` … `v0.15`. They sort below every CalVer
tag, so both schemes coexist without confusing `git tag --sort=version:refname`.

### Why this shape

Two hard constraints, and `vYY.M.BUILD` satisfies both with no remapping:

**GoReleaser requires semver.** It [enforces semantic versioning and errors on
non-compliant tags](https://goreleaser.com/limitations/semver/), and every rela
artifact is built by GoReleaser. `vYY.M.BUILD` is valid semver with all three
fields carrying real values, so `prerelease: auto` and nfpm's `-alpha`
extraction read correct major/minor/patch numbers.

**Windows MSI caps the version fields.** `ProductVersion` is
`major.minor.build` with maxima **255 / 255 / 65535**. `YY <= 99` and `M <= 12`
sit far inside those limits, so the tag goes into the installer verbatim — no
second "MSI version" to derive and keep in sync.

This second constraint is what rules out
[openvwr's](https://github.com/Sourcehaven-BV/openvwr) `vYYYYMMDD`, the scheme
rela's is adapted from. That format is actually fine for GoReleaser —
`v20260725` parses as major=`20260725`, orders correctly, and extracts `-alpha`
properly — but the `20260725` major blows the MSI 255 cap and `wix build`
fails. openvwr never hits this because it ships a single PHP `tar.gz` where the
version is, in its own words, "a package label baked into the archive" — never
a parsed version field. rela ships `.msi`, `.dmg`, `.deb` and `.rpm`
installers, where it is.

The month is not zero-padded because semver forbids leading zeros in a numeric
component (`v26.07.0` would be non-canonical).

## Cutting a release

Two equivalent options:

1. **Recommended — automated tag cut.** Run the
   [`Tag Release`](../.github/workflows/tag-release.yml) workflow from the
   Actions tab (`workflow_dispatch`). It computes the next CalVer tag, pushes
   it from `develop`, and the `Release` workflow takes over. This removes the
   manual step where the empty-release footgun lives.

   Inputs: `ref` (branch/commit, default `develop`), `alpha` (cut a `-alpha`
   prerelease), and `dry_run` (compute the tag without pushing — use this to
   preview what the next tag will be).

2. **Manual tag push.** From an up-to-date `develop`:

   ```bash
   TAG=$(./scripts/generate-version-tag.sh)
   git tag -a "$TAG" -m "Release $TAG" && git push origin "$TAG"
   ```

   Always let the script compute the tag; it scans existing tags so a second
   release in the same month increments `BUILD` instead of colliding.

### The tag-push trigger and the GitHub App token

A tag pushed by a workflow using the default `GITHUB_TOKEN` **does not fire
`on: push` events** — a GitHub safeguard against recursive workflow runs. If
`Tag Release` pushed with it, `Release` would never start.

So `Tag Release` pushes with a **GitHub App token**, minted at the top of the
job by `actions/create-github-app-token` from the existing `APP_ID` /
`APP_PRIVATE_KEY` secrets. A push authenticated that way does fire `on: push`,
so tagging and building stay one causal chain.

No new secret is needed: this is the same app `security.yml` and
`dependabot-auto-merge.yml` already use, and it already pushes branches and
opens PRs in `security.yml`, so it has the `contents: write` permission a tag
push requires.

If the app token is ever unavailable, `Release` also accepts a
`workflow_dispatch`, so a tag can still be built and published manually from
the Actions tab.

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
- **Release-runner drift from CI.** The `test` job in `release.yml` is a
  *second* copy of CI's test gate, so it can rot independently. It must
  keep matching `ci.yml`'s: `ubuntu-26.04` plus the bubblewrap install.
  External commands fail closed with no sandbox, so a runner without
  `bwrap` fails ~35 attachment/cmdexec/transform/export tests — on a commit
  that passed CI. This is what left `v26.7.1` a tag with no release.

## Verifying a release

After the workflow finishes, every release must contain both the default
and `postgres` archives per OS/arch, e.g.:

```bash
gh release view v26.7.0 --json assets --jq '[.assets[].name]'
```

Expected to include `rela_<ver>_<os>_<arch>.tar.gz` **and**
`rela-postgres_<ver>_<os>_<arch>.tar.gz` (`.zip` on Windows), plus the
desktop installers and `checksums.txt`.
