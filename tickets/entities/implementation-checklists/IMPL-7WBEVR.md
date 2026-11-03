---
id: IMPL-7WBEVR
type: implementation-checklist
title: 'Implementation: Release: CycloneDX SBOM per archive'
started: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: workflow steps only)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: workflow steps only; shell dry run below)
- [x] Happy path implemented (`sbom` job: download archives listed in `checksums.txt`, verify, syft scan each, upload `<archive>.cdx.json`)
- [x] Edge cases from planning handled (no archives in `checksums.txt` fails; checksum mismatch fails; SBOM without Go stdlib fails; dispatch from a branch uses GoReleaser's tag)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no tests)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no tests)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no tests)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no tests)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] ~~Each acceptance criterion verified with test scenario from planning~~ (N/A: no planning checklist)
- [x] Edge cases manually verified

**Verification Evidence:**

- The pinned sha256 matches the `syft_1.54.1_linux_amd64.tar.gz` line of upstream `syft_1.54.1_checksums.txt`.
- The "Install syft" run block, extracted with yq, downloaded the linux tarball and passed `sha256sum -c`.
- The "Generate and attach SBOMs" run block, extracted from `release.yml` with yq, ran with `TAG=v26.10.14`, syft 1.54.1 and a `gh` wrapper that passes downloads through and only prints uploads. All 18 archives (tar.gz and zip) passed the checksum check and the stdlib guard; the upload call listed 18 `.cdx.json` files.
- Locally built `rela-server-postgres` (linux/amd64, `-trimpath -tags=postgres`): 73 components, every library with version and purl, `pkg:golang/stdlib@1.26.9`.
- `rela-postgres_26.10.14_linux_amd64.tar.gz` (sha256 matches the fleet pin): 176 libraries, 109 unique purls; OSV querybatch accepts all of them.
- The stdlib guard returns non-zero on an SBOM without a stdlib component.

## Quality

- [x] Code follows project patterns (version pin and checksum check as in `security-scan.yml`)
- [x] ~~Checked for DRY opportunities~~ (N/A: two short steps)
- [x] No security issues introduced (no untrusted input in `run:`; least-privilege permissions)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
