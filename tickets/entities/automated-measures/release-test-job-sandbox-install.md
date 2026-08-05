---
id: release-test-job-sandbox-install
type: automated-measure
title: Release workflow Test job installs the command sandbox
description: SUPERSEDED by release-test-gate-sandbox-parity (BUG-2J30F3), which describes the same steps in release.yml and shipped first. Kept only so BUG-OJNWVK's adds-measure link resolves; do not treat as a second, independent guard.
kind: ci
location: .github/workflows/release.yml
status: deprecated
---

Superseded by [[release-test-gate-sandbox-parity]].

Both entities describe the **same** steps in `release.yml` — pin `ubuntu-26.04`,
install bubblewrap, keep the `bwrap --unshare-all --ro-bind / / /bin/true`
probe. BUG-2J30F3's version landed on `develop` first (PR #1256) and is the one
actually in the workflow; this entity records the duplicate discovery via
[[BUG-OJNWVK]] and is retained only so that bug's `adds-measure` link resolves.

There is one guard in the file, not two.
