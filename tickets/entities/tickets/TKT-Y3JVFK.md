---
id: TKT-Y3JVFK
type: ticket
title: 'Aggregate-over-hidden-rows documents: elevated document renders whose output is a derived statistic'
kind: enhancement
priority: medium
effort: l
status: review
---

## Problem

Some reports must compute over rows the reader may not see.

Concrete shape: sales managers each own a client set and are disallowed
**entirely** on the others — not merely denied the revenue figures, but denied
the *existence* of those clients ("don't even know they are in the system").
That is the row-level rule working as designed: a hidden entity is nonexistent.

Now the operator wants a report that **benchmarks a manager company-wide**, or
against the best-performing manager. The manager may legitimately learn "you are
at 60% of the top performer" while still being unable to learn who that
performer is, which clients they hold, or that those clients exist at all.

**No `acl.yaml` role can express this.** Row-level ACL is all-or-nothing per
row: granting enough to COMPUTE the benchmark grants enough to ENUMERATE the
competitors. The requirement is "may aggregate over, may not see", which is not
a read-privilege statement about an identity and therefore falls outside the
mechanism DEC-O59WM4 designates (see RR-6K3G8Q, addressed).

Today a document render reads through the ACL-gated
`lua.ReadDeps.VisibleReader`, so such a report computes only over the caller's
own rows and silently reports a benchmark against nothing.

## The security claim this rests on

**The aggregation is the confidentiality boundary.** What reaches the manager is
a derived statistic — a total, a rank, a percentage — not the rows behind it.
The report is safe to share precisely BECAUSE the Lua aggregates.

This is a stronger and more fragile claim than "the ACL bounds the output",
because nothing in the system enforces it. An elevated script that prints its
inputs instead of a statistic is an unbounded read oracle (RR-LWD8N3). The whole
design therefore turns on: **who is trusted to write these scripts, and how is
that trust made visible to the operator?**

Note this also brings the classic aggregation-disclosure problem in scope: a
benchmark over a SMALL peer group can identify individuals (if a manager is one
of two, "the other one earned X" is exact disclosure). Whether v1 addresses
k-anonymity or merely documents the hazard is an open decision, but it must not
be silently ignored.

## Approach: read-only elevation (decided — RES-XZBZXB, Option A)

RR-5IFZ23 raised this as a critical blocker; research showed it was overstated.

**Document renders are already writer runtimes.** `App.luaWriteDeps()`
(`app.go:321`) sets `EntityManager`, and `runDocumentScript`
(`list_document.go:85`) goes `NewWriterRuntime` -> `lua.NewWriter` ->
`newRuntime(allowWrites=true)`. So the `if allowWrites` guard is already true
here; only `deps.ElevatedManager != nil` (`runtime.go:713`) blocks.

Read and write elevation are already structurally separate inside `lua`:
`registerElevatedReads` (`runtime.go:1951`) takes only `er EntityReader`, and
`readGuard` already checks `er == nil` independently of the mutator. The
`luascriptrunner.go:149-160` coupling is on the CASCADE path, which document
renders never touch. The stated invariant ("read elevation cannot outlive or
precede write elevation") guarantees elevation never appears without operator
opt-in — it does not require reads and writes to be granted together.

**Decided shape:**

1. Register `bypass_acl` when EITHER `ElevatedManager` or `ElevatedReader` is
non-nil (`runtime.go:713`), and relax `luaBypassACL`'s defensive raise
accordingly.
2. `newElevatedHandle` registers the three write methods ONLY when `em != nil`.
A document render's `admin` table therefore exposes exactly `get_entity`,
`list_entities`, `get_relations`; `admin.delete_entity` is "attempt to call a
nil value". Reads-only is structural, not promised.
3. An operator opt-in on the `documents:` entry wiring `script.ReadElevation`
into the render.
4. `permission:` REQUIRED on any elevated document — necessary but not
sufficient, see RR-JE2G14.

Closure scoping, self-invalidation, and the `acl-bypass-read` audit record are
reused unchanged.

**Residual risk:** this widens a deliberately narrow gate, so a future wiring
site could grant reads without writes accidentally. Mitigations: nil-reader
stays a DENY (never a fallback to the gated reader), the document opt-in must be
explicit config, and a test pins that no write method exists on a read-only
handle.

**Rejected:** Option D (precompute an aggregate entity via a scheduled job) — it
forces the operator to model an aggregate type and keep a job in sync to obtain
what is fundamentally a caching property. Precomputation is an optimization rela
may apply internally later (RR-1DV8RY notes elevated renders are
principal-independent, hence uniquely cacheable); it does not belong in the
domain model. Option B (a second `rela.elevated_read` binding) — contradicts the
one-mechanism reasoning of TKT-ACSBSA and DEC-O59WM4. Option C (an aggregation
primitive that never returns rows) — the only option making RR-LWD8N3
structural, but disproportionate here; roadmap if the pattern recurs.

## Constraints to resolve in planning

- **RR-5IFZ23 (addressed)** — approach decided above. Residual risk carries into
implementation: keep nil-reader = DENY (never a fallback to the gated reader),
require explicit document config for the opt-in, and pin with a test asserting
no write method exists on a read-only handle.
- **RR-LWD8N3 (significant, open)** — the read-oracle exposure. An elevated
document's script is TRUSTED CODE whose blast radius equals the union of what it
reads. Same shape as RR-37AYC0. Decide where that trust is stated for the
operator, and whether `acl-bypass-read` (which records the read, not what
reached the page) is adequate accounting.
- **RR-JE2G14 (significant, open)** — fails open under NopACL / `--read-only`,
where `HoldsPermission` returns true. An elevated document with no configured
policy must not silently serve. Also: validate the permission string, and decide
the `--read-only` arm explicitly.
- **RR-1DV8RY (minor, open)** — elevated renders are principal-independent,
hence uniquely cacheable and uniquely poisonable. Relates to TKT-OGR566 and
RR-P4E9GL.
- **DEC-O59WM4** states data-entry-invoked scripts read as the request
principal. This is an operator-opted exception to that row and likely warrants
its own decision entity rather than a silent widening.
- **Read-path Lua.** `bypass_acl` is write-time only today. Pointing it at a read
surface needs its own justification, not the cascade's.
- **Aggregation disclosure / k-anonymity** — small peer groups leak individuals.

## Also in scope: fix the `permission:` godoc

`DocumentConfig.Permission` (`config.go:683-699`) justifies itself as "documents
whose COMPOSITION is sensitive even though the parts are individually readable"
— a confidentiality argument that does NOT hold under gated reads, where
`VisibleReader` already bounds the content. Under this ticket the field gains
its real second meaning. Per the project rule ("write down which of the two you
mean"), state the conditional rationale: with gated reads `permission:` guards
against a report claiming a scope it did not compute; with elevated reads it IS
the confidentiality boundary.

## Non-goals

- Elevated WRITES from a document render (explicitly rejected, see Blocker).
- Runtime-wide or ambient read elevation (DEC-O59WM4).
- Changing the fail-open default for non-elevated documents (RR-ZXGPCU).
- A `public: true` escape hatch.

## References

- TKT-M1AX6P (standalone documents), RR-ZXGPCU (documents fail open by design)
- TKT-D8T148, TKT-ACSBSA (bypass_acl + elevated reads), TKT-1WV50C
- DEC-O59WM4, RR-37AYC0 (command payload read oracle), TKT-OGR566
- `internal/lua/runtime.go:709-718`, `internal/script/luascriptrunner.go:149-160`
- `internal/script/list_document.go:58`, `internal/dataentry/standalone_document_handler.go:39`
