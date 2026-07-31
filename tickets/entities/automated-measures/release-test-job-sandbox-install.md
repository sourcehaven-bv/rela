---
id: release-test-job-sandbox-install
type: automated-measure
title: Release workflow Test job installs the command sandbox
description: The Release workflow's Test job installs bubblewrap before running `go test -race ./...`, mirroring the equivalent step in ci.yml. Without it internal/cmdexec fails closed, ~20 cmdexec/attachment/transform tests fail, and the Release job is skipped so nothing is published (BUG-OJNWVK).
kind: ci
location: .github/workflows/release.yml
status: active
---

Keeps the Release workflow's Test job able to run the command-sandbox tests.

`internal/cmdexec` fails closed by design: with no sandbox mechanism on the
host, commands refuse to run. That makes bubblewrap a hard **test dependency**
on Linux, not an optional extra — every command-running test fails without it.

`ci.yml` has installed it since the sandbox landed; `release.yml` never did, and
because `release.yml` only fires on tag pushes the drift stayed invisible until
a release was attempted.

Related: [[release-embedded-spa-guard]] guards the same workflow against a
different silent failure. Both share a root cause — `release.yml` re-declares
setup that `ci.yml` and the justfile already express, and nothing keeps the
copies in sync.
