---
id: BUG-VUNO11
type: bug
title: MCP update_entity ignores field-level write grants
description: The golden corpus recorded MCP update_entity writing a field outside the caller's field grant; the test fixture used AllowAllFieldGate, while production refuses the write.
priority: high
why1: The corpus case BUG-VUNO11-mcp-update-denied-field recorded the editor's write of note as allowed.
why2: 'appbuildtest.New wired FieldGate: AllowAllFieldGate{} even when a Declarative ACL was configured.'
why3: The fixture's entitymanager.Deps literal is a hand-copy of buildEntityManager's; when TKT-0XL8MF added the field gate, the fixture took the opt-out instead of the production builder.
why4: The field gate builder was unexported, so a fixture could not reach the production path and the opt-out was the only option that compiled.
why5: Authorization deps in the fixture are only partly sourced from shared production builders (TransitionWiring); the rest drift silently because nothing compares fixture wiring to production wiring.
prevention: The fixture builds the gate through appbuild.BuildFieldWriteGate keyed on its resolved ACL; a production-wired MCP test (TestRemoteMCP_UpdateEntityHonorsFieldGrants) pins the refusal independently of the fixture.
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

## Description

The TKT-0M4H0S golden corpus recorded MCP `update_entity` writing the `note`
field for the editor, whose `fields:` grant leaves `note` out. REST refuses the
same write with 403. Production does not have this gap: since TKT-0XL8MF the
remote MCP server writes through `entitymanager.FieldGated`, and a test through
`appbuild.New` and `dataentrywire.RemoteMCPServer` confirms the refusal. The gap
was in the test fixture. `appbuildtest.New` wired `FieldGate:
entitymanager.AllowAllFieldGate{}` even with a Declarative ACL, so the corpus
pinned a write that production refuses.

## Threat model

The corpus is the wire contract for the access layer: a refactor that changes
who may read or write what must show up as a golden diff. That only holds if the
corpus app is authorized the way rela-server is.

- A fixture that is more permissive than production hides a regression. If the
production MCP handle lost its field gate, the corpus would still pass, because
it already recorded the write as allowed.
- A recorded "allowed" also misleads a reader into treating a real denial as a
known gap, or into "fixing" production to match the corpus.
- The fix must not invent a second gate path in the fixture. It must build the
gate through the same function production uses, keyed on the same ACL value as
the transition guard, so the two cannot drift.
- Nothing here widens access. The only expected golden changes are denials that
production also gives, plus read counts of the gate itself.

## Fix

`appbuild.BuildFieldWriteGate` returns the gate production's `buildFieldPolicy`
builds (no flow edit ceiling). The fixture uses it with its resolved ACL, so a
Declarative ACL gets the policy gate and NopACL or ReadOnlyACL keep
AllowAllFieldGate. The corpus is re-recorded.
`TestRemoteMCP_UpdateEntityHonorsFieldGrants` pins the production refusal.
