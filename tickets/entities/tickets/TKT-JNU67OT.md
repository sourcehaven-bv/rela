---
id: TKT-JNU67OT
type: ticket
title: Reduce Go and TypeScript code size
kind: chore
priority: low
effort: m
started: "2026-11-27"
status: review
---

## Description

Reduce Go and TypeScript code size with clean, behavior-preserving idioms. No
code golf.

## Scope

- Go: delete code that `deadcode -test` reports dead under every build tag; move `sort.*` to `slices`/`cmp`; apply `go fix` modernizers; merge near-identical tests into table-driven tests.
- Frontend: remove what `knip` reports unused (files, exports, dependencies); `withDefaults` to props destructure; `defineModel` for v-model controls; value-mapping `switch` to lookup tables.
- Out of scope: behavior changes, new abstractions, sorts that need new compare helpers, vendored `third_party/`.

## Acceptance criteria

1. Build, vet and lint gates pass for every build tag.
2. Go and frontend test suites pass unchanged in meaning.
3. No test named in docs or CLAUDE.md is removed; race-gated tests stay top-level.
