---
id: FEAT-S31FF7
type: feature
title: Release versioning and automated tag cutting
description: Date-based (CalVer) release versioning with a workflow-dispatch tag cutter that computes and pushes the next tag, so releases are cut from the Actions tab rather than by hand.
status: proposed
---

Releases are versioned with a CalVer scheme (`vYY.M.BUILD`) and cut by a
manually-triggered `Tag Release` workflow that computes the next tag, pushes it,
and lets the existing `Release` workflow build and publish.

The scheme must satisfy two independent constraints:

- **GoReleaser requires semantic versioning** and errors on non-compliant
tags. Every rela artifact is built by GoReleaser.
- **Windows MSI `ProductVersion`** is `major.minor.build` with maxima
255 / 255 / 65535, so a four-digit-year major is illegal.

`vYY.M.BUILD` satisfies both with no remapping, keeping one version string
across every artifact (archives, DMG, MSI, deb, rpm).

Cutting releases from a workflow rather than by hand is also what prevents the
empty-release failure mode (a GitHub release object created in the UI before the
tag exists, leaving a published-looking release with zero assets).
