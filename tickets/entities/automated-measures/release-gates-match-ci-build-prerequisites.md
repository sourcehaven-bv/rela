---
id: release-gates-match-ci-build-prerequisites
type: automated-measure
title: The release workflow's gate jobs carry the same build prerequisites as CI
description: Pins that every workflow which compiles the full package tree installs the same system build dependencies. release.yml's Test and Security jobs duplicate ci.yml's and security.yml's gates by copy-paste, so a new toolchain prerequisite added to one and not the others breaks the release path while develop stays green. This has now happened twice (bubblewrap for v26.7.1, GTK4/WebKitGTK for v26.9.2-.4).
kind: test
location: .github/workflows/release.yml, .github/workflows/ci.yml, .github/workflows/security.yml
status: proposed
---

## What this catches

A workflow that compiles the tree without the system libraries the tree needs.
The defect class is not "GTK headers are missing from release.yml" specifically,
it is that the build prerequisites are declared three times by copy-paste, so
the copies can drift and only the one that runs least often notices.

The failure is silent in the worst way: `go test ./...` exits 1 with **no test
failing**, because a single package in the pattern could not be built. The same
input makes govulncheck exit 1 at package load. Both read as a red gate with
nothing obviously wrong in the test output.

## Why this is not just "add the package"

Adding `libgtk-4-dev` to release.yml fixes this occurrence and leaves the
mechanism intact. The next system dependency will be added to ci.yml by
whichever PR needs it, CI will go green, and the release will break on the tag
after that. The v26.7.1 incident is recorded in a comment in release.yml itself,
and the comment did not prevent the recurrence -- prose next to one of three
copies is not a constraint.

## What the measure must pin

**One definition of the gate, not three.** The durable fix is to extract the
test and security gates into a reusable workflow (`workflow_call`) that both
ci.yml and release.yml invoke. Then there is no copy to drift: a prerequisite
added for CI is automatically present at release time. This is the preferred
form of the measure.

**Failing that, an assertion that the copies agree.** If the jobs stay
duplicated, a check must compare the `apt-get install` package sets across the
workflows that run `go build`/`go test` over the full tree and fail when they
diverge. This is weaker -- it pins the symptom rather than removing the
mechanism -- but it is testable today and would have caught both incidents.

**The release path must be exercisable before a tag.** A workflow whose first
execution is the release itself cannot fail early. Whatever form the fix takes,
there should be a way to run the release gates on a PR (the existing
`staging-build.yml` is the natural place) so that drift surfaces while it is
still cheap to fix.

## Scope note

`status: proposed`, not `active`. The immediate fix in this bug's PR restores the
release path but does not implement the measure -- that is a follow-up, and
recording it as proposed is what keeps the structural work visible rather than
letting the green build imply the class is closed.
