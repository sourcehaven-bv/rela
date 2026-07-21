---
id: TKT-JO2SAD
type: ticket
title: 'ACL doc-fields: RoleDef.description + top-level policy description (rela-docs phase 1b)'
kind: enhancement
priority: medium
effort: s
status: done
---

Add doc-oriented prose fields to the ACL policy so the future `rela docs`
generator can narrate the role model for **operator/support** documentation: a
`description` on `RoleDef` (what a role is for, in plain language) and an
optional top-level `description` on `Policy` (what the deployment's access model
is about). All additive, optional, backward-compatible — mirrors the metamodel
doc-fields shipped in phase 1a (TKT-0YBFT8). No behavior change: the fields are
prose read by the doc generator, never consulted by the authorization path.
Populate them in the in-tree example ACL policy so the generator has real
content.

This is **Phase 1b** of the rela-docs generator arc (RES-EK7LSA / FEAT-G4VO53).
Phase 1a added metamodel doc-fields; phase 2 is the generator itself.

## Scope

**IN:**
- `RoleDef.Description string` (`yaml:"description,omitempty"`) — internal/acl/policy.go
- `Policy.Description string` (`yaml:"description,omitempty"`) + add `"description"` to `knownPolicyKeys` (the tolerant loader's allowlist — without it the new key emits a spurious "unknown key" `slog.Warn`)
- populate an in-tree example `acl.yaml` with role + policy descriptions
- tests: parse / absence / round-trip / unknown-key-warning-suppressed

**OUT:**
- any wire/API exposure of roles (no dataentry endpoint reads `RoleDef` today — the phase-2 generator reads `Policy` directly)
- the generator itself (phase 2)
- ACL behavior changes of any kind

## Notes / grounding

- ACL's loader is **tolerant** (unknown keys warn-and-continue, unlike the metamodel loader which hard-rejects), so this is even lower-risk than phase 1a. The only loader touch is extending `knownPolicyKeys` so the new top-level key is silent.
- `RoleDef` is a nested map value, no allowlist gate — the field just needs the struct tag.
- `Policy.Validate()` is untouched: descriptions are never validated (prose).

## Acceptance criteria

- **AC1** `RoleDef.description` parses and round-trips.
- **AC2** `Policy` top-level `description` parses and round-trips.
- **AC3** the new top-level `description` key does NOT trigger the loader's unknown-key warning (added to `knownPolicyKeys`).
- **AC4** backward-compat: a policy without either field loads byte-identically to before; `Validate()` unaffected.
- **AC5** example `acl.yaml` populated with role + policy descriptions; loads clean.
- **AC6** tests cover each field + the unknown-key suppression.
