---
id: PLAN-1DETM0
type: planning-checklist
title: 'Planning: Aggregate-over-hidden-rows documents: elevated document renders whose output is a derived statistic'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:

1. Read-only elevation in `internal/lua`: register `rela.bypass_acl` when EITHER
`ElevatedManager` or `ElevatedReader` is non-nil; `newElevatedHandle` registers
write methods ONLY when `em != nil`.
1a. `allow_acl_bypass` becomes a string enum (`read`|`write`|`read+write`) on
BOTH automation actions and documents, plus an `internal/migration` step
rewriting `true` -> `read+write`. See Decisions.
2. A `documents:` opt-in that wires `script.ReadElevation` into a render.
3. A closed-switch authorization gate for elevated documents, mirroring
`authorizeCommand` (see Approach) — this is what makes `permission:` real.
4. Config validation: elevated ⇒ `permission:` required.
5. The elevated read audit (`acl-bypass-read`) reaching the document path.
6. Docs: `docs/lua-scripting.md`, `docs/data-entry.md`, `docs/acl-security.md`,
the `DocumentConfig.Permission` godoc rewrite.

OUT of scope:

- Elevated WRITES from a document render (the handle must not carry them).
- An aggregation primitive that structurally cannot return rows (Option C in
RES-XZBZXB) — the roadmap answer to RR-LWD8N3, not this ticket.
- Precomputation/caching of elevated renders (RR-1DV8RY; see TKT-OGR566).
- Removing write bindings from document mode generally (TKT-PX5YL7).
- k-anonymity enforcement (documented as a hazard here, not enforced).

**Acceptance Criteria:**

1. **A read-only elevated handle exposes no write methods.** In a runtime with
`ElevatedReader` set and `ElevatedManager` nil, `rela.bypass_acl` EXISTS and its
`admin` table has `get_entity`/`list_entities`/`get_relations`; `admin.delete_entity`,
`create_relation`, `delete_relation` are nil. *Test:* `internal/lua` unit test
asserting `attempt to call a nil value` for each write method.

2. **The cascade path is unchanged.** With both handles set, `admin` still
carries reads AND writes. *Test:* existing TKT-D8T148/ACSBSA tests stay green,
plus an explicit both-handles case.

3. **Nil reader remains a DENY, never a fallback.** With `ElevatedManager` set
and `ElevatedReader` nil, `admin.get_entity` raises "no elevated reader is
configured" and does NOT silently read through the gated `VisibleReader`.
*Test:* exists in spirit today; pin it explicitly (this is the property that
stops a partial graph being mistaken for a complete one).

4. **A standalone document declaring the opt-in renders with elevation.** Its
script reads an entity the request principal cannot see, and the render
succeeds. *Test:* `internal/dataentry` handler test with a Declarative ACL
hiding the row.

5. **A document WITHOUT the opt-in has no `bypass_acl` binding at all.**
*Test:* script calling `rela.bypass_acl` fails with "attempt to call a nil
value"; asserts elevation is opt-in, not ambient.

6. **Elevated + no `permission:` is a config error at load.** *Test:*
`validate_test.go`, mirroring `TestValidateConfig_Documents`.

7. **The gate is closed-switch over the ACL implementation, and denies under
`ReadOnlyACL` and unknown implementations.** *Test:* table-driven over
NopACL / ReadOnlyACL (value AND pointer forms) / *Declarative (nil and non-nil)
/ a stub unknown impl. This is the RR-CWWJGW canary shape.

8. **Under NopACL an elevated document does NOT serve.** See Security — this is
the one place the elevated gate deliberately DIVERGES from `authorizeCommand`,
whose NopACL arm grants. *Test:* asserts 403 (or refuses at load), and that the
renderer is not invoked.

9. **A denied principal never reaches the renderer.** *Test:* renderer call
count == 0 on deny, not merely a non-200 status.

10. **An elevated read from a document emits one `acl-bypass-read` audit row**
naming the principal and the document. *Test:* audit sink assertion.

11. **A failing elevated render still audits.** A script that reads then raises
leaves the audit row (the existing `defer` guarantees this — pin it on the
document path). *Test:* script raises after `admin.get_entity`.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-XZBZXB (done) — decoupling read from write elevation.
Options A-D surveyed; **Option A decided** by the operator. D (precompute an
aggregate entity via a scheduled job) rejected as a manual mess that pushes a
caching property into the domain model.

**Existing Solutions:**

- **`authorizeCommand` (`commands.go:84-119`) is the template for the gate.** A
closed switch on the ACL IMPLEMENTATION, not on the read gate, because
`readGateFromContext` returns `nopReadGate` under both NopACL and ReadOnlyACL
and its `HoldsPermission` returns **true** (`readgate.go:135`) — so a predicate
written against the gate alone FAILS OPEN. That was live bug RR-CWWJGW. The
current `gateDocumentPermission` (`standalone_document_handler.go:39`) is
written exactly that way; for a non-elevated document that is harmless (the
content is ACL-bounded anyway), but for an elevated one it is the whole
boundary.
- **`permitsNavEntry` / `permitsGatedUIElement` (`views_handler.go:354`)** is
the OTHER shape — UX filtering only, explicitly "never what to allow". The
elevated gate must NOT be modelled on it.
- **`registerElevatedReads` (`runtime.go:1951`)** already takes only
`er EntityReader`; `readGuard` already checks `er == nil` independently of the
mutator. The read/write split inside `lua` is already structural.
- **`lua.NewReader` (`runtime.go:307`)** shows a read-only runtime posture is
first-class (relevant to TKT-PX5YL7, not required here).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Lua layer* (`internal/lua`)

- `registerBindings` (`runtime.go:711-718`): change the condition from
`r.deps.ElevatedManager != nil` to `... != nil || r.deps.ElevatedReader != nil`.
- `luaBypassACL` (`runtime.go:1775`): the defensive raise becomes "no elevated
handle is available", triggered only when BOTH are nil.
- `newElevatedHandle`: wrap the three write `SetField` calls in `if em != nil`.
Reads are already conditioned via `readGuard`. Result: a read-only elevation
yields an `admin` table with exactly three methods.
- `WriteDeps.ElevatedReader` godoc: state that it may now be set WITHOUT
`ElevatedManager`, and that doing so is a read-only elevation.

*Script layer* (`internal/script`)

- `runDocumentScript` (`list_document.go:58`) gains an optional
`ReadElevation`, threaded from the document service. `ExecuteDocument` /
`ExecuteStandaloneDocument` keep their typed-seam shape (no variadic
`lua.Option`), so a caller cannot forge elevation.

*Config layer* (`internal/dataentryconfig`)

- `DocumentConfig` gains the opt-in field (name to be settled — see Open
Questions).
- `validateDocuments`: elevated ⇒ `permission:` non-empty, else config error.

*Data-entry layer* (`internal/dataentry`)

- **New `authorizeElevatedDocument(ctx, aclImpl, docCfg) bool`**, a closed
switch mirroring `authorizeCommand`, used IN ADDITION to the existing
`gateDocumentPermission` for elevated documents only:
  - `nil` ACL → deny (wiring bug fails closed).
  - `NopACL` → **deny** (diverges from `authorizeCommand`; see Security).
  - `ReadOnlyACL` (value AND face) → deny.
  - `*Declarative` → nil-check, then `Permission != ""` AND held.
  - `default` → deny.
- The gate runs BEFORE the renderer, preserving the existing
gate-before-render ordering.

**Files to modify:**

- `internal/lua/runtime.go`, `internal/lua/deps.go`
- `internal/script/list_document.go`, `internal/script/executor.go`
- `internal/dataentryconfig/config.go`, `validate.go`
- `internal/dataentry/standalone_document_handler.go`, `document.go`, `app.go`
- `internal/appbuild/` (supply the elevation bundle to the document service)
- `docs/lua-scripting.md`, `docs/data-entry.md`, `docs/acl-security.md`
- `internal/dataentry/CLAUDE.md` (one line: elevated documents)

**Alternatives considered:** see RES-XZBZXB — Option B (second binding,
rejected: two mechanisms), Option C (aggregation primitive, roadmap), Option D
(precompute, rejected by operator).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**The central risk (RR-LWD8N3, accepted-with-mitigation):** under Option A the
elevated script is TRUSTED CODE. `bypass_acl` hands it a raw reader; nothing
stops it printing what it reads. So `permission: report:sales` does not mean
"may view this report" — it means **"may read whatever this script reads"**.
This is the same shape as RR-37AYC0 (command payloads as a read oracle).

Mitigation is documentation + a narrow gate, NOT enforcement:

- `docs/acl-security.md` gains an explicit statement of the trust boundary.
- The `DocumentConfig.Permission` godoc states the conditional rationale:
gated reads ⇒ guards against a report claiming a scope it did not compute;
elevated reads ⇒ IS the confidentiality boundary.
- Enforcement would require Option C; recorded as the roadmap answer.

**Why NopACL DENIES here (divergence from `authorizeCommand`):**
`authorizeCommand`'s NopACL arm grants, to preserve pre-ACL behavior — defensible
because that behavior predates the gate. An elevated document has NO pre-ACL
behavior to preserve: the feature is new, so "preserve the old semantics" grants
nothing and merely creates a configuration in which the only boundary is inert.
With no policy, `permission:` names a capability nothing can withhold
(`nopReadGate.HoldsPermission` ⇒ true), so the document would serve
company-wide data to every caller. Denying costs an operator who wants elevation
one `acl.yaml`; granting silently publishes the data. **Preference: refuse at
config load** ("elevated documents require a configured acl.yaml") over a
runtime 403 — an invalid configuration should not start.

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
|---|---|---|---|
| opt-in flag | operator config | bool; requires `permission:` | config error at load |
| `permission:` value | operator config | non-empty when elevated; ideally known to `acl.yaml` | config error |
| `docName` path segment | HTTP client | `isSafePathSegment` (existing) | 400 |
| ACL implementation | wiring | closed switch, default DENY | deny |

**Security-Sensitive Operations:**

- **Elevated read** — closure-scoped, self-invalidating, audited
(`acl-bypass-read`, once per closure, on BOTH the success and raise paths).
- **Renderer invocation** — must be unreachable on deny (AC9).
- **Error responses** — a denied elevated document must not distinguish itself
from a denied ordinary one in a way that reveals whether elevation is
configured. Note the config-is-not-secret rule means the doc NAME need not be
concealed; a 403 naming the missing permission is correct and useful.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** each AC above names its layer and assertion. Layers:

- `internal/lua` — AC1, 2, 3 (handle shape; the load-bearing unit tests).
- `internal/dataentryconfig/validate_test.go` — AC6.
- `internal/dataentry` — AC4, 5, 7, 8, 9 (handler + gate), AC10, 11 (audit).

**Edge Cases:**

- Both handles nil → no `bypass_acl` binding (today's behavior, must not change).
- Reader set, manager nil → read-only handle (the new case).
- Manager set, reader nil → writes work, reads raise (AC3).
- `&acl.ReadOnlyACL{}` pointer form → deny (the `&`-bypass RR-CWWJGW guarded).
- Nil `*acl.Declarative` → deny.
- An ACL implementation with no arm → deny (default).
- Elevated doc + `entity_type:` set (entity-anchored) → ALLOWED; both the
per-entity read gate and `authorizeElevatedDocument` must pass. Test both kinds.
- `allow_acl_bypass: true` (legacy bool) in an unmigrated metamodel → loud
config error, never a silent reinterpretation.
- `allow_acl_bypass: write` on a document → config error (a render has no write
capability); `read` on an automation action → reads elevate, writes do not.
- Script captures `admin` into a global and uses it after the closure → raises
(existing `live` guard; pin on the document path).
- Elevated render that raises mid-way → audit row still written (AC11).
- Two concurrent elevated renders → no shared-state leak between runtimes.
- Small peer group (k-anonymity) → NOT enforced; documented hazard.

**Negative Tests:**

- Denied principal: renderer not invoked, no partial output flushed.
- No acl.yaml + elevated doc → refuses (AC8).
- Elevated doc with empty `permission:` → config error (AC6).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| **Elevated script echoes raw rows** (RR-LWD8N3) | **High** | Not enforceable under Option A; trust boundary documented in acl-security.md + godoc. Option C is the structural answer if the pattern recurs |
| **Gate written against the read gate fails open** (RR-CWWJGW/RR-JE2G14) | **High** | Closed switch on ACL impl, mirroring authorizeCommand; table test over every impl incl. face forms |
| **NopACL deployment serves elevated docs ungated** | **High** | Refuse at config load; AC8 |
| Widening `bypass_acl` registration lets a future wiring site grant reads accidentally | Medium | nil-reader stays DENY; opt-in must be explicit config; AC1/AC3 pin the handle shape |
| Elevated renders are expensive and unbounded | Medium | Pre-existing (RR-P4E9GL, TKT-OGR566); note, do not solve here |
| Caching an elevated render later poisons the gated view | Medium | RR-1DV8RY; any future cache must key on elevation posture |
| Aggregation discloses individuals in small groups | Medium | Documented hazard; k-anonymity out of scope |

**Effort:** l (raised from m: the allow_acl_bypass enum + migration is additional scope)

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/lua-scripting.md` — read-only elevation; `bypass_acl` in documents
- [x] `docs/data-entry.md` — the opt-in, and why `permission:` is mandatory
- [x] `docs/acl-security.md` — the trust boundary (elevated script = trusted code)
- [x] `internal/dataentryconfig/config.go` — `Permission` godoc rewrite
- [x] `internal/dataentry/CLAUDE.md` — one line
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI commands change)
- [x] ~~`docs/schema.md`~~ (N/A: the document opt-in lives in `data-entry.yaml`; note the `allow_acl_bypass` enum DOES touch the metamodel's automation actions, covered by the migration docs)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- RR-5IFZ23 (critical) — **addressed**: blocker overstated; Option A decided.
- RR-6K3G8Q (significant) — **addressed**: acl.yaml cannot express
aggregate-over-hidden-rows.
- RR-LWD8N3 (significant) — **addressed**: documented-not-enforced, operator
confirmed. Trust boundary stated in acl-security.md + the Permission godoc.
- RR-JE2G14 (significant) — **addressed**: closed-switch `authorizeElevatedDocument`
mirroring `authorizeCommand`, plus NopACL refusal at config load.
- RR-1DV8RY (minor) — **deferred**: caching out of scope; the key-on-elevation-
posture constraint is recorded for whoever adds it.

## Decisions (operator-confirmed)

1. **`allow_acl_bypass` becomes a string enum: `read` | `write` | `read+write`.**
It is currently a `bool` on `metamodel.AutomationAction` (`types.go:651`),
threaded through `automation` -> `autocascade` -> `script` with ONE decision site
(`luascriptrunner.go:152`). A document declares `allow_acl_bypass: read`.

   **No bool accepted; a migration rewrites `true` -> `read+write`.** Chosen as
the tech-debt-free end state:
   - A boolean names the exception, not the posture — the same objection
DEC-O59WM4 raised against `unrestricted_reads: true`. `true` means "read+write"
only by historical accident, not because the config says so.
   - A dual-accepting unmarshaller has no forcing function that ever removes it,
so two representations of one concept persist forever and every reader
(validation, docs, tooling, the next mode added) handles both.
   - This field grants ACL bypass. A permissive parser that silently maps a
legacy value to the BROADEST setting is the wrong default for a privilege field;
if the representations drift, the shim resolves toward more access. Failing
loudly on an unmigrated `true` shows the operator a clear error instead of a
silently reinterpreted grant.
   - `internal/migration` already exists for project YAML and the rewrite is
mechanical with no judgment calls.

   `write`-only gains no consumer today but is included: the enum grades one
axis, and omitting a value to avoid an unused case makes the vocabulary
lopsided. Handle assembly is already `if em != nil` / `if er != nil`, so it
costs nothing.

   **What the flag is FOR (operator intent, and the reason three values
suffice).** `allow_acl_bypass` is a ROUGH GUARD — a triage marker for review,
not a fine-grained permission model. It answers one question for whoever
deploys: *does this script need its bypass block read carefully before it
ships?*

   - No bypass ⇒ the script can only do what the invoking user can. No special
scrutiny, no audit trail needed — the ACL already bounds it.
   - Bypass ⇒ a human reads that closure before deployment.

   The value tells the reviewer WHICH KIND of scrutiny (is this reading data the
user cannot see, or writing past their permissions?), which is all the review
decision needs. Verb-level granularity (`create`, `update`, `delete`) would add
config surface without changing what the reviewer does, so it is deliberately
NOT modelled.

   **`create` was considered and rejected for v1.** `newElevatedHandle` exposes
only `create_relation` / `delete_relation` / `delete_entity` — there is no
elevated `create_entity` or `update_entity` (the godoc at `runtime.go:1852`
defers them as "a larger surface best added with its own tests"). So
`allow_acl_bypass: create` would name a capability the system does not have,
the exact failure DEC-O59WM4 rejected in the four-value `...-mode` enum:
"config that names a nonexistent capability is worse than a missing field: it
appears to work." `create` is also a different AXIS (a verb within write) from
read-vs-write, so mixing them makes the value set ambiguous. Revisit only if
elevated entity creation is implemented; at that point a set-valued form
(`allow_acl_bypass: [read, write]`) would let verbs be added without a second
migration.

   This framing also underwrites the RR-LWD8N3 decision: human review of the
bypass block IS the mitigation, which is precisely why the docs must state that
an elevated script is trusted code.

2. **RR-LWD8N3: accepted as documented-not-enforced.** `docs/acl-security.md`
states the trust boundary — an elevated document's script is TRUSTED CODE, and
its `permission:` grants read access to everything the script reads, not merely
to the report. Option C (an aggregation primitive that structurally cannot
return rows) remains the roadmap answer if the pattern recurs.

3. **Entity-anchored documents MAY be elevated.** Both gates apply: the existing
per-entity read gate AND `authorizeElevatedDocument`. Elevation is not
standalone-only. Test matrix covers both kinds.

4. **NopACL: refuse at config load** ("elevated documents require a configured
acl.yaml") rather than a runtime 403 — an invalid configuration should not
start.
