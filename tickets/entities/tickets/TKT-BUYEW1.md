---
id: TKT-BUYEW1
type: ticket
title: 'Apply field-level visible: redaction to the appbuild gated read paths'
kind: enhancement
priority: medium
effort: s
status: done
---

Closes the field-level half of read-side ACL on the appbuild-wired
identity-bearing read paths. Split out of [[TKT-0XL8MF]], which retains the
write-side `FieldWriteGate` work.

## Problem

`appbuild` could not construct an affordance resolver, so these paths were ROW
gated only: a caller who could read an entity at all received **every** property
of it, including ones a human with the same role sees redacted in the UI.

| Site | Ticket / RR | Documented? |
|---|---|---|
| `Services.ScheduledLuaWriteDeps` | RR-7408F5 | KNOWN LIMITATION note |
| `Services.GatedReads` | [[TKT-UIR41P]] | KNOWN LIMITATION note |
| cascade `lua.ReadDeps` | — | **no** — found in code review |

The third site is the sharpest: it carried no note at all, so nothing warned a
reader it was unenforced. All three send data onward (a prompt, a webhook, an
automation write), so an unredacted property leaves the system entirely.

## Approach

The plumbing already existed — every site threaded a `visibility.FieldRedactor`
and `visibility.PolicyRedactor` already adapted `*affordances.PolicyResolver`.
The missing piece was only the value. Build the resolver once during assembly
(`buildFieldRedactor`) and pass it where `nil` used to go.

- Adds the `appbuild -> affordances` arch-lint edge. `affordances` is a
near-leaf and imports no wiring package, so the edge cannot close a cycle.
- The resolver is a **private field, not an accessor**: `Services` is at its
plimsoll exported-method ceiling, and `WithMachines` is the one mutator on an
otherwise-immutable resolver, safe only before the value is shared.
- `NopRedactor` for both "no policy" and "policy without affordance grants" —
ordinary configurations, so the no-policy path stays byte-identical. A policy
that declares grants but fails to compile is an **error**, not a fallback:
degrading there would silently serve unredacted data to an operator who asked
for redaction (RR-GKCZO5).

## Scope limit (deliberate)

Field redaction covers **entity properties only**. Relation meta is not
redacted: `gatedGraphReader.GetRelation` reads raw, and relation-level
`visible:` grants (`acl.RelationGrant.Visible`, honored on the dataentry wire)
are not consulted here. The `GatedReads` godoc states this rather than implying
broader coverage.

## Acceptance

- A scheduled job, an MCP read, and an automation cascade all receive
`visible:`-redacted properties.
- Both KNOWN LIMITATION godoc notes are deleted, each with a test that fails
without the fix.
- A project with no `acl.yaml` reads byte-identically to before.
- A row-only policy (no `visible:` grants) redacts nothing.
- `just arch-lint` passes.

## Evidence

Four regression tests, **all mutation-verified** (reverting the redactor to nil
makes each fail on its own assertion):

| Test | Pins |
|---|---|
| `TestGatedReads_RedactsHiddenField` | MCP single-entity read |
| `TestGatedReads_RedactsOnListPath` | list path — separate code, higher volume |
| `TestScheduledLuaWriteDeps_RedactsHiddenField` | scheduler path (RR-7408F5) |
| `TestCascadeReadDeps_RedactsHiddenField` | cascade, end-to-end through real Lua |

Plus two controls (`TestNoPolicy_RedactorHidesNothing`,
`TestPolicyWithoutAffordanceGrants_HidesNothing`) so the set cannot pass by
over-redacting.

The cascade test drives a real `PatchEntity` → automation → Lua action that
copies the hidden salary onto an unrestricted field. Without the fix it lands
`"99000"` — the leak, demonstrated rather than argued.

Also corrects `docs-project/entities/guides/GUIDE-scheduled-tasks.md` and its
generated output, which told operators field policy did not apply.
