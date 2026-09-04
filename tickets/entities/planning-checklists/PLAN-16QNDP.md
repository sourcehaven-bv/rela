---
id: PLAN-16QNDP
type: planning-checklist
title: 'Planning: Lua scripts cannot distinguish an ACL-redacted property from a genuinely-unset one'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:

- A carrier on the redacted `entity.Entity` recording which property
names field-level ACL withheld, populated at `visibility.Redact`.
- Lua surfacing of that carrier via `EntityToTable`.
- Parity with the existing `_redacted` HTTP wire field (DEC-T0XIWQ).

OUT of scope:

- Body/content redaction — still not policy-expressible; the existing
`TODO(body-redaction)` in `policyreader.go:162` stands untouched.
- Relation field redaction — relations have no field-level redaction
today (RR-BZNL0S).
- Changing *what* is stripped. Property values stay hidden; only the
*fact* of redaction becomes observable.
- Row-level gating. A hidden entity stays nonexistent to Lua.

**Acceptance Criteria:**

1. A Lua script reading an entity with an ACL-hidden property can
distinguish it from an unset property. *Test:* two properties on one entity —
one hidden by `visible:`, one genuinely absent. Script asserts the hidden one
reports redacted and the absent one does not.
2. The hidden property's VALUE is still unreachable from Lua.
*Test:* assert `properties.<hidden>` is nil and the value string appears nowhere
in the serialized table.
3. Nothing-hidden reads are unchanged, allocation and shape identical.
*Test:* `NopRedactor` path — no redaction marker present, and `Redact` still
returns the ORIGINAL face.
4. The validator does not skip entities that merely have an
ACL-redacted property. *Test:* validator over a gated reader with a hidden
property still evaluates rules for that entity.
5. The data-entry write path does not report an ACL-redacted entity as
git-crypt "inaccessible". *Test:* PUT an entity with a hidden property → not a
422 `encrypted_inaccessible`.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art
- [x] ~~Run `/research`~~ (N/A: single well-bounded seam, prior art
already exists in-tree)

**Research Doc:** N/A — the design question is narrow and the in-codebase prior
art is decisive.

**Existing Solutions:**

- **DEC-T0XIWQ / `redactedPropertyNames`**
(`internal/dataentry/affordances.go:846`) — the HTTP surface already ships a
`_redacted` field listing withheld property names to the SPA. This is the
precedent that settles the confidentiality question, and the shape to mirror: a
**sorted list of names**, always non-nil, where empty means "evaluated, nothing
redacted".
- **`entity.Inaccessible []InaccessibleField`** — considered and
REJECTED as the carrier. See Alternatives below; this is the most important
research finding.
- **`visibility.Redact`** (`policyreader.go:184`) — the single choke
point every Lua read path passes through (`ScriptReader.GetEntity`,
`ListEntities`, and the pushdown `RedactRow`). One edit covers all.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Add a **new, separate field** on `entity.Entity` for ACL-redacted property names
— do NOT reuse `Inaccessible`:

```go
// Redacted names the properties withheld from the reading principal
// by field-level ACL (`visible:`). Distinct from Inaccessible: a
// redacted property is readable BY SOMEONE, the stored file is intact,
// and the entity is writable. Read-out artifact only — never persisted.
Redacted []string `json:"redacted,omitempty"`
```

Populate it in `visibility.Redact`, the one choke point, from the `hidden` set
it already computes — sorted for determinism, mirroring `redactedPropertyNames`.
The nothing-hidden early return is unchanged, so the no-policy path stays
allocation-free and byte-identical.

Expose it in `EntityToTable` (`internal/lua/runtime.go:1300`) as both:

- `entity.redacted` — a name-keyed set table (`{salary = true}`),
cheap to iterate and to test membership.
- `entity:is_redacted(name)` — a method matching the existing
`prop` / `strip_prefix` idiom.

Both, because they serve different scripts: a report iterating all fields wants
the set; a template checking one field wants the method.

**Alternatives considered:**

- **Reuse `entity.Inaccessible`** — rejected, despite its godoc
explicitly inviting "Lua-driven access control". Investigation found
`IsLocked()` is `len(Inaccessible) > 0` and is a **write-blocker and a
skip-condition** at six sites. Two would actively break:
  - `internal/validator/validator.go:198` silently `continue`s past
locked entities. The validator reads through `lateGatedReader`
(`internal/dataentry/app.go:742`), i.e. the redacting path — so every entity
with any ACL-hidden property would be **silently dropped from validation**. A
silent correctness hole.
  - `internal/dataentry/write_handler.go:335` returns 422
`encrypted_inaccessible` with "run `git-crypt unlock` first" — a nonsense
message for an ACL redaction, and a spurious write block.

The two concepts genuinely differ: `Inaccessible` means *nobody here can read
this file, do not write through it*; ACL redaction means *this principal may not
see this value, but the data is intact and writable*. Conflating them buys a
shared field and costs a silent validation bug. The godoc invitation should be
**corrected** as part of this ticket.
- **A separate Lua-only side channel** (e.g. a second return value from
the bindings) — rejected: it would have to be threaded through `GetEntity`,
`ListEntities`, search hydration and markdown entity-refs independently, and
each new read binding could forget it. Putting it on the entity keeps the fact
attached to the thing it describes.

**Files to modify:**

- `internal/entity/entity.go` — add `Redacted`; extend `Clone`; correct
the `InaccessibleReason` godoc that currently invites this misuse.
- `internal/visibility/policyreader.go` — populate in `Redact`.
- `internal/lua/runtime.go` — `EntityToTable` + `luaEntityIsRedacted`.
- `docs/` — the Lua binding reference for the new entity fields.
- Tests as listed in the Test Plan.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

No new external input. The only new data flow is *outward*: property NAMES that
the ACL policy already evaluated move from the redactor into the script's view.
Names originate from the operator-authored metamodel, not from a caller.

**Security-Sensitive Operations:**

This ticket **widens** what a script observes, so it is a deliberate disclosure
decision, justified as follows:

- Root CLAUDE.md: field-level `visible:` redaction hides property
**values only**; it makes "no claim to conceal which properties exist", because
the metamodel is served over `/api/v1/_schema`. A field-existence oracle is
explicitly not a threat guarded against.
- DEC-T0XIWQ already discloses exactly this list over HTTP. Lua is a
*narrower* audience than the SPA, so this is parity, not expansion.

Invariants that MUST be preserved (each gets a test):

- Property **values** remain unreachable — only names surface.
- **Row-level** gating is untouched: a denied entity is still
`store.ErrNotFound`, indistinguishable from a genuine miss.
- **Write-prep reads stay raw.** `ReadDeps.WritePrepStore` is unchanged;
`luaUpdateEntity` must keep reading unredacted, or hidden properties are erased
on save. Pinned already by `TestScriptReads_UpdatePreservesHiddenProperties`.
- **Never persisted.** `Redacted` is a per-reader artifact. It must not
reach markdown frontmatter, and `internal/canonical` must ignore it exactly as
it already ignores `Inaccessible` (`canonical.go:62`) — otherwise two principals
compute different content hashes for the same entity.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**

| AC | Test |
|----|------|
| 1 | `internal/lua` script test: entity with hidden + absent property; assert redacted set contains only the hidden name |
| 2 | Assert hidden VALUE absent from the Lua table and from any serialization of it |
| 3 | `NopRedactor` path: assert `Redact` returns the original face and no marker appears |
| 4 | `internal/validator`: gated reader, entity with hidden property, assert rules still evaluate (regression guard for the rejected alternative) |
| 5 | `internal/dataentry`: PUT with a hidden property, assert not 422 `encrypted_inaccessible` |

Integration: an end-to-end script test through the real
`visibility.ScriptReader` + `DeclarativeGate` wiring (not a stub redactor), so
the choke point is exercised as wired.

**Edge Cases:**

- Nothing hidden → empty/absent marker, original face returned.
- ALL properties hidden → fail-closed redactor returns every name;
script sees an empty `properties` and a full redacted set.
- Entity that is BOTH git-crypt inaccessible AND ACL-redacted — the two
carriers must coexist without either being lost, and `IsLocked()` must reflect
only the git-crypt one.
- Property hidden by ACL whose name collides with the reserved
`content` sentinel.
- Pushdown path (`RedactRow`) vs. `Filter` path must produce identical
markers — easy to fix one and miss the other.
- Fail-closed redactor error path: must mark redacted, never silently
reveal.
- `AllowAllReader` — no redaction, no marker, byte-identical.

**Negative Tests:**

- A script must NOT be able to read a hidden value via the new marker,
`entity:prop()`, or the raw properties table.
- A redacted entity must NOT be treated as locked by write paths.
- The marker must NOT survive a round-trip to markdown storage.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated (m)

**Risks:**

| Risk | Mitigation |
|------|-----------|
| Reusing `Inaccessible` silently breaks validation (validator skips locked entities read through the gated reader) | Avoided by design — separate field. AC 4 is the regression guard. A future author may repeat the mistake, so the misleading `InaccessibleReason` godoc gets corrected in this ticket. |
| Marker leaks into persisted markdown or the content hash | Explicit tests; `internal/canonical` already excludes `Inaccessible` and must exclude this too. |
| Pushdown and Filter paths diverge | Test both; they share `Redact`, which is why the population goes there and not in the callers. |
| Widening disclosure beyond property names | Tests assert values never surface; row-gating untouched. |

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] Lua binding reference — document the new entity table fields and
the `is_redacted` method, including the explicit statement that names are
disclosed but values are not.
- [x] `docs/acl-security.md` — record that field redaction is
name-observable to scripts, consistent with the existing `_redacted` wire field.
- [x] Godoc on `entity.InaccessibleReason` — correct the "Lua-driven
access control" invitation, which points a future author at the rejected design.
- [x] ~~`docs/metamodel.md`, `docs/cli-reference.md`, README~~ (N/A: no
metamodel, CLI, or project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-79L852, RR-Q2ZRSP, RR-0A3JYK, RR-1G0T3F,
RR-KBWJPV (plus RR-IHWEB0, raised later during manual verification).

Two of the six were WITHDRAWN after checking them against the code rather than
against the plan's assumptions:

- **RR-Q2ZRSP** (three-way absent/empty/false branching) — withdrawn.
  Automations ARE ACL-bound (`LuaReadDepsFor` names them explicitly; the
  scheduler uses `ScheduledLuaWriteDeps`; data-entry uses the gated
  `scriptReader`). The ungated `LuaWriteDeps()` has only CLI/MCP/flow callers,
  which are operator-trust-boundary paths. "Unevaluated" is a property of the
  RUNTIME, not of an individual entity, so a script can never encounter a mix
  and branch usefully. Plain always-present set instead.
- **RR-0A3JYK** ("ACL down" indistinguishable from all-hidden) — withdrawn as
  factually wrong. `FieldRedactor`'s fail-closed godoc is an implementer
  CONTRACT, not a live code path: `PolicyResolver.FieldVerdicts` returns a value
  with no error channel, a nil policy yields empty verdicts, and malformed
  policy fails at `predicate.Compile` during construction. No runtime state
  produces the claimed ambiguity.

The three that held up were all verified directly in code rather than inferred
from a comment — and RR-KBWJPV found a real bug (see the implementation
checklist).
