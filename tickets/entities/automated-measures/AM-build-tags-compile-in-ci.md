---
id: AM-build-tags-compile-in-ci
type: automated-measure
title: Every build tag that guards Go files is compiled in CI
description: CI builds/vets each build tag that guards Go source (notably `e2e`), so a tag-guarded file cannot silently stop compiling while every visible gate stays green.
kind: ci
location: .github/workflows/ci.yml (build-tag compile matrix)
status: proposed
---

## What it prevents

[[BUG-J3YJCC]]: `internal/dataentry/e2e_test.go` drifted three parameters behind
`NewApp` and stopped compiling under `-tags e2e`. Nothing noticed, because **no
CI job builds that tag** — the default build excludes the file, so the signal
that would normally catch a signature change (a red build) was never produced.

The failure mode is specific to tag-guarded code: the more thoroughly a tag
isolates a file from the default build, the more completely it also isolates it
from the default build's safety net. A tag that is never compiled is a directory
of code with no gate at all, and its rot is invisible until someone tries to use
it — typically the person who needed it working right then.

## Shape of the measure

A compile/vet step per guarded tag (`e2e`, plus any future tag introduced for Go
sources). Compilation alone is sufficient — this catches signature drift, which
is the whole failure class; it does not require the tagged tests to actually
run, which may need external services.

Note the existing backend-selection tags (`postgres`, `memorybackend`, `sqlite`)
are already built by the cross-compile and postgres jobs, so this measure is
about closing the remaining gaps rather than starting from zero.
