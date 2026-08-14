---
id: MEAS-PRINCIPAL-RESTAMP
type: automated-measure
title: Principal re-stamp sites preserve verified claims
description: A test that every context re-stamp of a Principal (changing Tool while keeping identity) preserves orgID, orgSlug, roles and RawUser. Guards the class of bug where a plain composite literal silently zeroes the unexported verified-assertion fields.
kind: test
location: internal/dataentry/sync_handlers_test.go + internal/principal/
status: proposed
---

## What it guards

The trust-boundary design in `internal/principal` (unexported claim fields +
`Verified()` as sole constructor) makes *forging* a claim impossible but makes
*dropping* one easy and silent — a composite literal compiles fine and zeroes
them.

`resolvePrincipalEntity` (`internal/dataentry/router.go:380-382`) carries a
prose comment warning about this. Prose does not fail CI; `syncContext`
(`sync_handlers.go:20-23`) has the same shape and the bug (BUG-0Q8MCZ).

## Shape

A table-driven test over the re-stamp sites asserting that a Principal built by
`principal.Verified(...)` with non-empty org/slug/roles/RawUser survives the
re-stamp with only `Tool` changed.

Stronger variant worth considering: if the fix introduces a single blessed
helper (e.g. `principal.WithTool`), test the helper directly and add a lint or
grep-based guard that no other site constructs a `principal.Principal{...}`
literal from an existing principal. That converts a convention into a structural
guarantee — the same move TKT-80EWGM made for `PatchEntity` when it removed the
raw write-prep handle rather than documenting against it.

## Why automated rather than review

The failure is invisible at the call site: the code compiles, tests pass, and
the only symptom is an authorization grant quietly not applying in a deployment
that uses asserted roles. Reviewers have already missed it once.
