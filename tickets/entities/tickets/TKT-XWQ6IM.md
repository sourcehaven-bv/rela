---
id: TKT-XWQ6IM
type: ticket
title: 'Release: CycloneDX SBOM per archive'
kind: chore
priority: medium
effort: xs
started: "2026-10-21"
completed: "2026-10-21"
status: done
description: The release job attaches a CycloneDX JSON SBOM made by a pinned syft to each GoReleaser archive. Atlas uses it for a nightly OSV check of the deployed release.
---

## Description

Sourcehaven wants a nightly atlas job that reads the SBOM of each deployed rela
release and queries OSV for known vulnerabilities. The fleet deploys
`rela-postgres_<ver>_linux_amd64.tar.gz`. No release has an SBOM yet.

The `release` job now runs syft on every GoReleaser archive and attaches
`<archive>.cdx.json` to the release. syft reads the build info embedded in each
Go binary, so the SBOM lists the module versions and the Go stdlib
(`pkg:golang/stdlib`) that ship.

syft is pinned by version and by the sha256 of its release tarball. The workflow
default permission drops to `contents: read`; only the `release` and `desktop`
jobs get `contents: write`.

## Acceptance criteria

1. Each GoReleaser archive gets a CycloneDX JSON SBOM named `<archive>.cdx.json` on the GitHub release.
2. The SBOM lists Go modules and the Go stdlib with versions and purls; the job fails if the stdlib is missing.
3. syft is pinned by version and tarball sha256.
4. Jobs that do not publish run with `contents: read`.
