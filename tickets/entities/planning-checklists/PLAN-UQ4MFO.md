---
id: PLAN-UQ4MFO
type: planning-checklist
title: 'Planning: Permission-based navigation filtering (UX: hide menu entries a user cannot use)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:

1. Optional `permission:` on `NavigationEntry`.
2. `/api/v1/_sidebar` omits entries whose `permission:` the principal lacks.
3. A group left empty by that filtering is dropped entirely.
4. Docs: `docs-project/.../GUIDE-data-entry.md` (nav table + the field) and
`GUIDE-acl-security.md` (reconcile § "Sidebar menu structure is
principal-independent" with the new behaviour).

OUT of scope — each with a reason, because each was actively considered:

- **`permission:` on `List`/`Kanban`/`Action`.** Discussed with the user and
deferred. For lists/kanbans the ACL already does the real work — rows are scoped
by `scopedSortedEntities`, so a denied user gets an empty list rather than a
refusal. A `permission:` there would govern only whether you may *look at the
page* while the data is governed by something else: two mechanisms on one
screen, and an invitation for someone to later mistake the label for a boundary.
For `Action` it is a genuine capability gate, but that belongs with the missing
enforcement (TKT-X06LA2), not here.
- **Filtering `/_config`.** Concealment-shaped and explicitly unwanted; the SPA
builds the menu from `SidebarData`, so `/_sidebar` alone achieves the UX goal.
- **Count-based hiding** ("hide a list whose count is 0"). Ruled out by the
user: counts are a separate problem (TKT-LP90MA).
- **Any change to counts or their staleness.** Same reason.
- **Gating `handleV1Action`.** TKT-X06LA2.

**Acceptance Criteria:**

1. **A nav entry with no `permission:` is always shown.** The overwhelmingly
common case must be untouched. *Test:* existing sidebar tests stay green
unmodified — `TestV1SidebarWithNavigation`, `TestACLSidebar_*`.

2. **Under NopACL (no `acl.yaml`), an entry with `permission:` is shown.**
No policy configured ⇒ no restrictions, matching `nopReadGate`'s allow-all
posture. *Test:* explicit case with a `permission:` entry and no ACL wired; also
keeps `TestACLSidebar_NopACLFullCounts` green.

3. **Under a configured policy, a holder sees the entry.** *Test:* alice holds
`admin:read`, sees the gated entry.

4. **Under a configured policy, a non-holder does not.** *Test:* bob lacks it,
the entry is absent from every group.

5. **Under `--read-only` (`ReadOnlyACL`), an entry with `permission:` is
hidden.** This is the arm that the read gate alone gets wrong (see Security).
*Test:* asserts hidden, and is the canary for the RR-CWWJGW hazard.

   **REVISED DURING REVIEW (RR-XYO03L): read-only now SHOWS gated entries.**
   The rationale above was wrong and I had not checked it against
   `acl.ReadOnlyACL`'s contract: it implements only `AuthorizeWrite` and
   restricts no reads, while nav entries are overwhelmingly read surfaces. It
   also carries no identity, so hiding removed entries from *everyone* rather
   than from non-holders. Read-only is structurally the same case as NopACL —
   no policy, so no permission model — and is grouped with it. The explicit arm
   remains, because falling through to the read gate would reach the same
   answer by accident (RR-CWWJGW); `TestNavPermission_ReadOnlyArmIsExplicit`
   pins the difference.

6. **A group whose every item is hidden is dropped**, not rendered as a bare
heading. *Test:* two-item group, both gated, both denied ⇒ group absent.

7. **A group that keeps at least one item is retained with only the survivors.**
*Test:* mixed group ⇒ group present, exactly the permitted items.

8. **Filtering is presentation only.** A hidden entry's target still behaves
exactly as before by direct URL. *Test:* asserts a denied principal's
`/list/:id` still returns its normal (ACL-scoped, possibly empty) response — the
filter changed no enforcement.

9. **`/_config` is unchanged** — still serves `navigation` in full to every
principal. *Test:* asserts config identical for holder and non-holder.

10. **Config validation rejects a `permission:` on a group entry** (a group is
a container, not a destination). *Test:* validation table case.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the mechanism is established in-tree; the open question
was a config-design decision, resolved with the user.

**Existing Solutions:**

- **`resolveCommands` / `authorizeCommand`** (`internal/dataentry/commands.go:130`,
`:84`) is the pattern to mirror: "return only what this principal can use, the
endpoint re-checks". Its doc comment states the invariant explicitly — *"This is
presentation only — `authorizeCommand` is re-consulted at exec time, which is
the actual boundary."* Three properties to copy:
  - a **closed switch** over the `acl.ACL` implementation (`commands.go:72-83`)
so a new implementation denies until an arm is added;
  - **both value and pointer forms matched** — `AuthorizeWrite` has a value
receiver, and matching only the value form once let `&acl.ReadOnlyACL{}` fall
into the granting default arm: a `--read-only` bypass reachable by one `&`;
  - the ACL implementation read **once** outside the loop (`commands.go:145`).
- **`_actions` on entity/list responses** is the same shape one level down, and
`internal/dataentry/CLAUDE.md` already carries the rule: "Don't trust `_actions`
for authorization. The write endpoint must re-authorize."
- **`readGate.HoldsPermission`** (`readgate.go:52`) is the predicate itself,
already used by the history path (`history_handler.go:124`).

**Why `ReadQuery` is NOT the mechanism** (checked, and it is a trap):
`readquery.go:49-51` returns `DenyAll` only when *no role in the entire policy*
grants read on the type. A principal holding no conferring edges gets a `Query`
that yields zero rows — not `DenyAll`. So `DenyAll` is **policy-shaped, not
principal-shaped** and cannot answer "can this user use this entry".
`TestACLSidebar_DenyAllZeroCounts` demonstrates exactly this: count 0 via
`Query`, not via `DenyAll`.

**Pre-existing issues noted, not fixed here:**

- `handleV1Action` has no per-principal gate at all → TKT-X06LA2 (filed).
- The sidebar is fetched once on mount and never refreshed → TKT-LP90MA (filed).
- `sidebarCountsByLabel` (`acl_sidebar_test.go:85`) only records items with a
non-nil `Count`, so it cannot distinguish "hidden" from "present with no count".
This ticket needs a sibling helper that returns labels/hrefs.
- `TestACLSidebar_DenyAllZeroCounts` asserts `counts[label] == 0`, which would
pass *vacuously* if the entry disappeared (missing key ⇒ zero value). It is not
affected by this change (those entries carry no `permission:`), but it should be
tightened to assert presence while it is being read anyway.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Config* (`internal/dataentryconfig`)

- `NavigationEntry` (`config.go:433`): add `Permission string`
(`yaml:"permission,omitempty"`), matching the field name commands and documents
already use.
- `validateNavEntry` (`validate.go:195`): reject `permission:` on a **group**
entry — a group is a container with no destination, and gating it would be
ambiguous with the empty-group drop. Groups vanish when all their children do.

*Sidebar* (`internal/dataentry/views_handler.go`)

- Add `permitsNavEntry(ctx, aclImpl, entry) bool`, mirroring `authorizeCommand`'s
closed switch:
  - `entry.Permission == ""` → **true** (the common case, short-circuited first);
  - `aclImpl == nil` → false (wiring bug fails closed);
  - `acl.NopACL` / `*acl.NopACL` → **true** (no policy ⇒ no restrictions, AC2);
  - `acl.ReadOnlyACL` / `*acl.ReadOnlyACL` → **show**, grouped with NopACL
(revised during review — see AC5). Originally specified as **hide**, on the
reasoning below, which was wrong: read-only means the
principal can't act; a permission-gated entry is exactly the thing to hide.
*This arm must be explicit and precede any read-gate use* — see Security;
  - `*acl.Declarative` → `readGateFromContext(ctx).HoldsPermission(...)`;
  - `default:` → false.
- `handleV1Sidebar` (`:192-211`): skip filtered items; after building a group's
items, `continue` if empty. Read the ACL implementation **once** before the
loop, as `resolveCommands` does.

*Docs* — both are GENERATED; edit `docs-project/entities/guides/*.md` then `just
docs` and diff.

**Files to modify:**

- `internal/dataentryconfig/config.go`, `validate.go` (+ `validate_test.go`)
- `internal/dataentry/views_handler.go` (+ `api_v1_test.go`, `acl_sidebar_test.go`)
- `docs-project/entities/guides/GUIDE-data-entry.md`, `GUIDE-acl-security.md`
- `frontend/src/types/config.ts` — add `permission?` to the TS `NavigationEntry`
for parity (the SPA reads `SidebarData`, so it needs no behaviour change)

**Alternatives considered:**

- *`permission:` on `List`/`Kanban`/`Action`* — see Scope. Deferred, not
rejected on difficulty; the door is left open below.
- *Derive from the target, with override* — the eventual shape if `List` ever
gains a real `permission:`. Adding it now would mean deriving from a field that
does not exist.
- *Reuse `ReadQuery`/`DenyAll`* — rejected; policy-shaped, see Research.
- *Hide on zero count* — ruled out by the user (TKT-LP90MA).
- *Filter `/_config` too* — concealment-shaped; explicitly unwanted.

**Future: derive from target.** If `List`/`Kanban`/`Action` later gain
`permission:` (TKT-X06LA2 proposes it for `Action`), the natural evolution is
derive-from-target with an explicit entry-level override. The entry-level field
added here is forward-compatible with that: it becomes the override.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**This feature is NOT a security control, and the code must not read as one.** A
hidden entry stays reachable by direct URL and its target behaves exactly as
before. The user's framing is the correct one: *guessing the URL gets you empty
stuff — no problem*, because lists are already row-scoped by the ACL.

Consequences for the implementation:

- **No concealment language** in code, docs, or tests. No test asserting a
config name is unenumerable. `/_config` keeps serving navigation in full. (Root
`CLAUDE.md`: "The configuration is not a secret; the data is." An earlier
attempt at this in TKT-M1AX6P was reverted for exactly this reason.)
- **`docs/acl-security.md` must be updated**, or it will contradict the code:
it currently records per-principal menu hiding as "deliberately not done". The
decision becomes "done for UX, explicitly not for confidentiality".

**The one real hazard: `ReadOnlyACL` fails OPEN through the read gate.**
`readGateFromContext` returns `nopReadGate` under **both** NopACL *and*
ReadOnlyACL (`readgate.go`), and `nopReadGate.HoldsPermission` returns `true`
unconditionally (`readgate.go:135`). So a predicate that consults only the read
gate would *show* every gated entry under `--read-only`. That combination was
live bug RR-CWWJGW, which is why `authorizeCommand` carries an explicit
ReadOnlyACL arm ahead of the gate. `permitsNavEntry` must do the same. AC5 is
the canary.

This is cosmetic here (a shown-but-unusable entry, not an escalation) — but the
same predicate shape gets copied, and the next copy may not be cosmetic.

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
|---|---|---|---|
| `permission:` on a nav entry | config author | non-empty string; rejected on a group entry | config error |
| principal | request context | existing ACL middleware | unchanged |

Not cross-checked against `acl.yaml` — matching the existing precedent for
command and document permissions, where the policy is not available at
config-validation time. A typo'd permission hides the entry (fail-closed under a
configured policy), which is the safe direction and visible immediately.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** as listed per AC. Layers:

- `internal/dataentryconfig/validate_test.go` — AC10.
- `internal/dataentry/acl_sidebar_test.go` — AC2-7, reusing `sidebarPolicy()`,
`installSidebarConfig`, `mustNewACL`, `gateCtxFor`, `aliceCtx`, `principalCtx`.
Needs a **new helper returning labels/hrefs**, since `sidebarCountsByLabel`
drops items with a nil count and so cannot express absence.
- `internal/dataentry/api_v1_test.go` — AC1 (existing tests unmodified), AC9.

**Edge Cases:**

- Entry with `permission:` **and** no ACL wired → shown (AC2).
- `--read-only` → hidden (AC5); explicitly not delegated to the read gate.
- Group with `permission:` → config error (AC10).
- Group emptied by filtering → dropped (AC6); group partially filtered →
retained with survivors (AC7).
- **Every** top-level entry filtered out → `navigation: []`. Confirm the SPA
renders an empty sidebar rather than erroring.
- A `permission:` naming a permission no role grants → entry hidden under a
configured policy, shown under NopACL. Consistent, and the typo is visible.
- Nav entry pointing at a nonexistent list/kanban → unchanged behaviour
(item with nil count); filtering is orthogonal.

**Negative Tests:**

- Non-holder: entry absent from `/_sidebar`, **and** the target endpoint
unchanged by direct request (AC8) — the assertion that keeps this honest as
presentation-only.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| ReadOnlyACL fails open via the read gate (RR-CWWJGW shape) | Medium | Explicit ReadOnlyACL arm before any gate call; AC5 pins it |
| The filter is later mistaken for a boundary | Medium | Endpoints unchanged and AC8 asserts it; no concealment language anywhere; `internal/dataentry/CLAUDE.md` gains a line |
| `acl-security.md` left contradicting the code | Medium | In scope; update the source guide, `just docs`, diff |
| Existing sidebar tests silently weakened (the vacuous-pass problem in `TestACLSidebar_DenyAllZeroCounts`) | Low-Med | New absence-asserting helper; tighten that test while in the file |
| Editing generated `docs/*.md` instead of the source | Low | Known trap (hit in TKT-M1AX6P); `just docs && git diff --stat docs/` is the check |

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `GUIDE-data-entry.md` — `permission:` in the navigation field table; a
short subsection stating it is a UX filter, that the target still enforces
independently, and the NopACL/read-only behaviour
- [x] `GUIDE-acl-security.md` — reconcile § "Sidebar menu structure is
principal-independent" with the new behaviour
- [x] `internal/dataentry/CLAUDE.md` — nav filtering is an affordance; the
ReadOnlyACL arm is load-bearing; do not filter `/_config`
- [x] ~~Root `CLAUDE.md`~~ (N/A: the config-is-not-secret rule added in
TKT-M1AX6P already covers the framing)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** No separate `/design-review` run. The design's two
load-bearing questions were resolved with the user directly (gate source; NopACL
and empty-group behaviour), and the survey that informed them was an explicit
code exploration rather than a guess. The one hazard a design review would look
for — the ReadOnlyACL fail-open — is identified above with its mitigation and a
dedicated AC.
