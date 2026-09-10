---
id: BUG-ONT37A
type: bug
title: The release workflow's gate jobs cannot build rela-desktop, so every tag since the Wails v3 migration produced no release
description: 'release.yml''s Test and Security jobs run on Linux without GTK4/WebKitGTK headers. cmd/rela-desktop links Wails v3 and fails to build without them, so `go test ./...` and govulncheck both exit 1 before doing any real work. Both gates are `needs:` of the Release job, which is therefore skipped and publishes nothing. v26.9.2, v26.9.3 and v26.9.4 exist as tags with no GitHub release. ci.yml and security.yml received the dependency install in PR #1546; release.yml did not.'
priority: high
effort: s
why1: The Test and Security jobs in release.yml fail with `Package 'gtk4' not found` / `Package 'webkitgtk-6.0' not found`, so `FAIL github.com/Sourcehaven-BV/rela/cmd/rela-desktop [build failed]` and govulncheck's `could not import C (no metadata for C)`. The Release job lists both in `needs:`, so it was skipped and no artifacts were built.
why2: The Linux runner has no GTK4/WebKitGTK development headers. cmd/rela-desktop links Wails v3, whose Linux build is cgo against those libraries, so the package cannot type-check without them -- which fails the whole `go test ./...` invocation and govulncheck's package load, neither of which is actually about the desktop app.
why3: 'PR #1546 (the Wails v3 migration) added `libgtk-4-dev libwebkitgtk-6.0-dev` to ci.yml and security.yml but not to release.yml, even though it edited all three files. The migration introduced a new build-time system dependency and only two of the three workflows that compile the tree learned about it.'
why4: Nothing forced the third workflow to be updated. The release workflow duplicates ci.yml's test and security gates as copy-pasted steps rather than sharing them, so a new toolchain prerequisite has to be remembered separately at each copy. CI stayed green on develop the whole time, which is exactly the signal a developer checks, so the divergence was invisible until a tag was pushed.
why5: 'The release path has no pre-merge feedback: it only ever runs on a tag push, after the code is already merged and the tag already created. A workflow whose first execution is the release itself has no way to fail early, so any drift between it and ci.yml is discovered by a failed release rather than by a failed PR. The identical failure had already happened once before (v26.7.1, recorded in release.yml''s own comment about bubblewrap) and the structural cause -- duplicated gate definitions with no shared source -- was not addressed then.'
prevention: 'Immediate fix: release.yml''s Test job installs `libgtk-4-dev libwebkitgtk-6.0-dev` alongside bubblewrap, and its Security job gains the same install step security.yml already has, so both mirror their ci.yml/security.yml counterparts. Structural follow-up worth doing: the release gates duplicate ci.yml rather than reusing it. Extracting the test and security jobs into a reusable workflow (`workflow_call`) that both ci.yml and release.yml invoke would make this class of drift impossible, since there would be one definition of the build prerequisites instead of three. That is the real fix -- this is the second occurrence of the same shape (v26.7.1 was the first, for bubblewrap), and a third is likely otherwise. Also worth noting: a failed release is currently silent, so the tags sat unreleased for two days; alerting on a failed release run would shorten discovery.'
status: done
---

## Summary

Pushing a release tag produces no release. The `Tag Release` workflow works
correctly and the `Release` workflow does fire, but its two gate jobs fail
before doing any useful work, so the job that actually builds and publishes
artifacts never runs.

Three tags are affected: **v26.9.2, v26.9.3, v26.9.4**. All exist as git tags
with no GitHub release and no assets.

## Evidence

Run 34500494895 (tag `v26.9.4`):

| Job | Result |
|-----|--------|
| Test | failure |
| Security | failure |
| Release | **skipped** |
| Desktop | skipped |
| Update Homebrew Cask | skipped |

Test job:

```text
Package 'gtk4' not found
Package 'webkitgtk-6.0' not found
FAIL github.com/Sourcehaven-BV/rela/cmd/rela-desktop [build failed]
```

Security job:

```text
govulncheck: loading packages:
Package 'glib-2.0', required by 'virtual:world', not found
could not import C (no metadata for C)
```

No test actually failed. `go test ./...` returns exit code 1 because one package
in the pattern could not be built.

## Why CI did not catch it

`ci.yml` and `security.yml` both install the GTK headers; `release.yml` does
not. All three were edited by PR #1546, so develop stays green while the release
path is broken. The release workflow only ever executes on a tag push, so there
is no earlier opportunity for it to fail.

## The rule

Every workflow that compiles the full package tree needs the same system build
dependencies. Today that contract is maintained by copy-paste across three
files, and this is the second time it has been broken the same way --
`release.yml`'s own comment records the previous occurrence, where a missing
bubblewrap install left v26.7.1 as a tag with no release.

## Fix

`release.yml` Test job installs `libgtk-4-dev libwebkitgtk-6.0-dev` with
bubblewrap; Security job gains the install step security.yml already carries.
Both now mirror their counterparts exactly.

The three affected tags can be republished after this lands.
