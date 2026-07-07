---
id: TKT-ELX09J
type: ticket
title: Apply whitelist-vs-struct parity tests to dataentryconfig + acl loaders
kind: test
priority: low
effort: xs
status: backlog
---

## Problem

BUG-5XIN07 was caused by a top-level-key whitelist (`validTopLevelKeys`)
drifting from the `Metamodel` struct's yaml tags — a new struct field was added
without its whitelist entry, so the loader rejected valid config. That bug added
a reflection-parity test (`TestValidTopLevelKeysMatchStruct`) to guard the
metamodel loader.

The **same hand-maintained-whitelist-vs-struct pattern** exists in two sibling
loaders with no parity guard:

- `internal/dataentryconfig/validate.go` (`validTopLevelKeys`)
- `internal/acl/policy.go` (`knownPolicyKeys`)

## What we want

Apply the same reflection-parity test to both: assert every top-level `yaml` tag
on the corresponding config struct is present in its whitelist, so a newly-added
struct field cannot be silently rejected by its own loader.

## Scope

- Add a parity test in each package mirroring `internal/metamodel/loader_test.go:TestValidTopLevelKeysMatchStruct`.
- Skip fields with empty or `-` yaml tags (computed fields).
- If either whitelist already diverges from its struct, fix the divergence (that would itself be a latent bug).

## Origin

Surfaced by the cranky-code-reviewer during BUG-5XIN07 review (RR-GHDQXC).
Prevention material for the 5-whys systemic cause.
