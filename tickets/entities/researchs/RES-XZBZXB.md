---
id: RES-XZBZXB
type: research
title: Decoupling read elevation from write elevation for document renders
summary: 'Read and write elevation are already structurally separate inside internal/lua, so decoupling them is two conditions rather than a redesign; Option A (register bypass_acl when either handle is wired, omit write methods when no Mutator) was chosen over precomputing aggregates, which would push a caching concern into the domain model.'
status: done
---

## Problem

TKT-Y3JVFK needs a document render whose Lua can read rows the caller cannot see
(benchmark a sales manager against peers whose clients are invisible to them),
while the rendered output is a derived statistic rather than the rows.

RR-5IFZ23 raised this as a critical blocker: `rela.bypass_acl` appeared welded
to elevated WRITES, which the ticket lists as a non-goal. This research
establishes what the coupling actually is and what decoupling would cost.

## Context — what the code actually does

**Finding 1 (corrects RR-5IFZ23): document renders are ALREADY writer
runtimes.**

`documentService` renders via `App.luaWriteDeps()` (`app.go:321`), which sets
`EntityManager: a.entityManager`. `runDocumentScript` (`list_document.go:85`)
calls `NewWriterRuntime` -> `lua.NewWriter` -> `newRuntime(...,
allowWrites=true)`.

So a document script today has the FULL write surface: `rela.create_entity`,
`update_entity`, `delete_entity`, `create_relation`, `delete_relation`,
`write_file`. `registerWriteBindings` has no `isDocument` guard. `isDocument`
gates only `print`, `rela.mode`/`rela.document.*`, and an `rela.output()`
warning.

RR-5IFZ23 said the `if allowWrites` condition was an obstacle. It is not — that
condition is already TRUE on this path. The real obstacle is only the second
condition, `deps.ElevatedManager != nil` (`runtime.go:713`).

**Finding 2: read and write elevation are already structurally separate.**

Inside `lua`, the split is clean:
- `registerElevatedReads` (`runtime.go:1951`) takes ONLY `er EntityReader` and
registers exactly the three read methods.
- `readGuard` (`runtime.go` in `newElevatedHandle`) already checks `er == nil`
independently of the mutator, raising a clear error.
- `WriteDeps.ElevatedReader` is documented as SEPARATE from `ElevatedManager`
and from `WritePrepStore`, with nil meaning DENY, not fallback.

The coupling lives at exactly two lines, both policy rather than structure:
1. `runtime.go:713` — `if r.deps.ElevatedManager != nil` gates REGISTRATION of
`bypass_acl` on the WRITE handle.
2. `luaBypassACL` (`runtime.go:1775`) — raises "no elevated write handle is
available" when `ElevatedManager == nil`, defensively.

Plus one on the script side: `luascriptrunner.go:149-160` sets `ElevatedReader`
only inside the `ElevatedProvider` branch. That is the CASCADE wiring site and
is irrelevant to documents, which never touch `LuaScriptRunner`.

**Finding 3: the invariant's stated rationale is about ordering, not fusion.**

`luascriptrunner.go:151-156` says read elevation "cannot outlive or precede
write elevation." Read precisely, that guarantees read elevation never appears
WITHOUT the two operator keys turning — it does not assert that reads are
meaningless alone. A read-only elevation reachable ONLY through its own explicit
operator opt-in satisfies the spirit (no accidental elevation) without the
letter (same branch as writes).

## Options

### Option A — read-only elevation as a first-class posture

Change the registration condition from `ElevatedManager != nil` to
`ElevatedManager != nil || ElevatedReader != nil`, and have `newElevatedHandle`
register write methods only when `em != nil`. `luaBypassACL`'s defensive raise
becomes "no elevated handle is available".

The `admin` table a document render receives then has `get_entity`,
`list_entities`, `get_relations` and NOTHING else — calling
`admin.delete_entity` is `attempt to call a nil value`, which is the strongest
possible guarantee that a render cannot mutate under elevation.

- Pros: smallest diff (~2 conditions + handle assembly); preserves closure
scoping, self-invalidation, and the `acl-bypass-read` audit unchanged; makes
"reads only" structural rather than a promise; the resulting capability is
strictly narrower than what cascade gets.
- Cons: widens a deliberately narrow gate — a future wiring site could grant
reads without writes by accident. Mitigate by keeping nil-reader = deny and
requiring the document opt-in to be explicit config, plus a test pinning that no
write method exists on a read-only handle.
- Effort: S.

### Option B — separate binding (`rela.elevated_read(fn)`)

Leave `bypass_acl` untouched; add a distinct read-only binding registered when
`ElevatedReader != nil` and `ElevatedManager == nil`.

- Pros: zero risk to the existing write path; the name states the capability.
- Cons: two mechanisms and two mental models, which DEC-O59WM4 and TKT-ACSBSA
both explicitly argued against ("one mechanism, one mental model"); duplicates
the liveness/audit/closure machinery or forces a shared helper anyway.
- Effort: M.

### Option C — do not elevate; add an aggregation primitive

Keep reads gated; give Lua a `rela.aggregate(...)` that computes counts/sums
over a type WITHOUT returning rows, evaluated below the visibility wrapper.

- Pros: the confidentiality property becomes STRUCTURAL — the script never holds
the hidden rows, so it cannot echo them. Directly answers RR-LWD8N3, which is
otherwise left as "trust the script author". Output is bounded by construction.
- Cons: much larger design (new query surface, aggregation grammar, per-backend
implementation); inflexible for real reports (the benchmark example wants
per-manager grouping, ranking, maybe ratios); does not compose with the existing
Lua idiom; still needs k-anonymity thinking.
- Effort: L/XL.

### Option D — precompute into a visible entity

A scheduled job (already permitted to read broadly via `run_as`) writes an
aggregate entity; the document renders that under normal gated reads.

- Pros: NO new capability at all — uses `run_as`, the existing sanctioned
mechanism (DEC-O59WM4), and the row-level ACL grants the report entity to the
right role. Confidentiality is structural: only the aggregate is ever stored.
Cacheable and cheap at render time.
- Cons: stale by the scheduler interval, not on-demand; needs a place to put the
aggregate entity and a metamodel type for it; awkward for per-caller
personalization ("YOUR rank") since the stored aggregate is principal-neutral —
though a hybrid works: precompute the company-wide part, render the caller's own
slice under gated reads.
- Effort: S (mostly config/metamodel, no engine change).

## Recommendation

**Option D first, Option A as the engine change if D is insufficient.**

Option D deserves serious weight because it needs no new capability, keeps
DEC-O59WM4 intact, and makes the confidentiality property structural rather than
script-dependent — the aggregate entity is the only thing that exists, so there
are no hidden rows to leak. For the stated benchmark use case a hybrid (stored
company-wide aggregate + caller's own gated slice) covers it, and staleness is
usually acceptable for a periodic report.

If on-demand freshness or per-caller aggregation genuinely rules D out, Option A
is the right engine change: it is small, it makes read-only elevation structural
(no write methods on the handle at all), and it reuses the audited,
closure-scoped machinery rather than inventing a parallel one. Option B's second
mechanism contradicts the explicit one-mechanism reasoning in TKT-ACSBSA and
DEC-O59WM4. Option C solves RR-LWD8N3 most thoroughly but is a disproportionate
build for this need; it belongs on the roadmap only if
aggregate-over-hidden-rows becomes a recurring pattern rather than one report.

**Neither A nor D resolves RR-LWD8N3 for A's path** — under Option A the script
is still trusted not to echo what it reads. That trust must be stated in the
docs and the operator must understand that `permission:` on an elevated document
grants "read whatever this script reads". Option D avoids the issue entirely,
which is its strongest argument.

## Incidental finding (separate ticket)

Document render scripts have the full write surface today (`create_entity`,
`delete_entity`, `write_file`) with no `isDocument` guard, on an HTTP GET.
Writes are ACL-gated and audited, so this is not a privilege escalation — but a
render that mutates on GET is surprising, is not idempotent, and interacts badly
with any future render cache (RR-1DV8RY). Worth its own ticket to decide whether
document mode should drop write bindings, independent of TKT-Y3JVFK.

## Recommendation (decided)

**Option A**, decided by the operator on review of this research.

Option D (precompute into a visible entity) was rejected on the grounds that it
is a manual mess: it forces the operator to model an aggregate entity type, wire
a scheduled job, and keep them in sync, in order to obtain what is fundamentally
a CACHING property. Precomputation of an expensive aggregate is an optimization
rela should be free to apply internally if and when it matters (see RR-1DV8RY —
an elevated render is principal-independent and therefore uniquely cacheable);
it should not be pushed into the domain model as a modelling obligation.

Option B is rejected for the reason TKT-ACSBSA and DEC-O59WM4 both give: one
mechanism, one mental model. Option C (an aggregation primitive) remains the
only option that makes RR-LWD8N3 structural rather than a trust statement, but
is disproportionate for this need; it stays on the roadmap if
aggregate-over-hidden-rows becomes a recurring pattern.

### Consequence carried forward

Under Option A the elevated script is TRUSTED CODE: it is trusted not to echo
the rows it reads. RR-LWD8N3 is therefore NOT resolved by this decision and must
be addressed as documentation + operator guidance in TKT-Y3JVFK, not silently
inherited. `permission:` on an elevated document grants "may read whatever this
script reads", and the operator must be told so plainly.
