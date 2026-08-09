---
id: RES-09YLLL
type: research
title: Declarative next-action layer
summary: 'Advisory one-at-a-time next-action layer: entity-shaped sources, operator-defined ordered bands with stable-random selection within a band (dwell time rejected), key = (source_id, entity_id, optional props), per-source mute + per-user snooze in a new user-state service with mem/disk/postgres backends. No caching — a cross-principal cache defeats the ACL gate. Empty-vs-absent already settled by internal/filter (prop= means empty). Set-shaped sources, per-entity mute, far-side predicates, date arithmetic and permission-filtered candidates all out of v1. Load-bearing gap is property predicates on store.GraphQuery (relation-shaped only today); Phase 0 needs no store changes.'
status: done
---

## Problem

Every system built on rela reimplements some version of "what should I do next?"
badly and bespoke. The request is an operator-configured declarative layer that
derives a **suggested** follow-up from graph state and surfaces it in the UI.

Framing that constrains everything below:

- **Advisory, not a task queue.** A hint, not a demand.
- **Things a user *could* do, not *should* do.** Some pressing, many merely
available. This distinguishes it from analysis/validation output, which has an
opinion about correctness.
- **One suggestion at a time.** The aim is a helpful companion, not to overload
the user. Not a todo list.
- **Good, not optimal.** Picking one of several good next actions is the goal;
avoiding a bad one is the bar. It does not need to pick the best.

The one-at-a-time constraint is load-bearing for the whole design — it is what
makes bounded candidate sets non-lossy, makes ranking explicability cheap, and
removes any "show me all 12" surface to grow into.

## Context — what already exists

Findings from reading the codebase (these corrected several premises in the
original request):

- **`today()` already exists** in `internal/predicatefns/predicatefns.go:48`,
returning a `DateType`; `internal/predicate` carries `Date` as a first-class
typed value with compile-time literal coercion. Date *arithmetic* (`days_since`)
does not exist — no Duration type, no date operators in the walker.
**Deliberately out of scope here** (belongs with pushing date conditions into
storage queries).
- **`store.GraphQueryer` is an existing pushdown seam** — SQL-native on pgstore
(`internal/store/pgstore/graphquery.go`, one recursive CTE + `WHERE EXISTS`,
streamed), with a `graphquerynaive` fallback for fs/mem, plus conformance and
bench suites. `GraphCount` and `MatchingIDs` alongside it.
- **`store.GraphQuery` is relation-shaped ONLY** (`internal/store/graphquery.go:20`):
`EntityType` + `HasInbound`/`HasOutbound` relation predicates with transitive
expansion. Built for the ACL read gate. **No property predicates. No negation.**
- **`search.SearchVisible` takes filters but cannot push them down** —
`internal/store/pgstore/visiblesearch.go:44-47` documents that `q.Filters` are
applied Go-side and the SQL `LIMIT` is dropped when filters are present.
Explicitly NOT the fast path.
- **`internal/affordances` is the ACL verdict resolver** — a name collision with
the request's `affordances:` key. Rename the config key.
- **`internal/analysis` + `validation.Violation`** already carry
`EntityID`/`EntityTitle`/`Description`/`Severity`/`RuleName` — close to a
suggestion payload. But validation runs Lua, and the root CLAUDE.md forbids
user-supplied Lua on the read path, so this can only ever be consumed from
cached/scheduled results, never evaluated live per page.
- **`DashboardCard` is closed** to `count|table|breakdown`
(`internal/dataentryconfig/config.go:626`). A `script:`-backed card would be the
ambient render surface; `DocumentConfig` (`config.go:739-759`) is the proven
template, including its discrimination-on-`EntityType` pattern.

## Core model (as settled in discussion)

### Sources

A source is a rule yielding 0..n **entity-shaped** candidates; the engine picks
one to show. Sources are pluggable and independent — none knows the others
exist. Adding a rule is adding a source; a bad source is deleted without
perturbing the rest.

Two kinds, discriminated by presence of `context:`, mirroring how `documents:`
discriminates standalone from entity-anchored on `entity_type:`:

| | Global | Context-aware |
|---|---|---|
| Key | no `context:` | `context: <entity_type>` |
| Candidates | `query:` | the entity being viewed |
| Renders on | dashboard | that entity's detail page |

### Bands, not numbers — operator-defined, ordered

Bands are declared **by the operator** in config as an ordered list; list order
is priority order. Sources reference a band by id; config validation rejects an
unknown reference.

Rationale for bands over numeric priorities: per-source numbers do not compose
(source A returns 90, source B returns 7, on different scales — arbitrary
ordering wearing the costume of ranking). For an advisory system,
**explicability beats optimality**: an unexplainable suggestion reads as broken
even when right.

**Bands are NOT hardcoded.** An ISMS deployment, a docs mirror and a consultancy
want different vocabularies; baking one set into the engine would force every
operator to translate their domain into ours — the same violation that
`metamodel.yaml` exists to prevent. Ship a reasonable default set in the starter
config, fully replaceable. A useful default:

`blocking` → `time-critical` → `stalled` → `tidying` → `ambient`

### Within a band: stable-random, NOT dwell time

Dwell-time ordering was considered and **rejected**. Pick randomly among the
eligible candidates in the winning band, seeded stably (per user per day) so a
refresh cannot re-roll it.

Why random is right rather than a compromise:

- **A crowded band is a configuration bug, not a ranking problem.** A well
configured system surfaces something only when there is actually something to
do. If a band routinely holds many simultaneous candidates, the operator has
written a rule that is too broad — and no clever tiebreak fixes that, it only
hides it. Random is honest about it, and the felt arbitrariness is the signal to
go fix the rule.
- **It is total.** Dwell time is undefined for content sources, undefined for
`count == 0` conditions, and awkward for anything without a clear onset. Random
works for every source kind.
- **It needs no state.** Dwell-time ordering requires knowing when each
condition became true — a stored onset or a property to derive one from. Random
removes that whole category of question.
- **It resists starvation.** Dwell ordering means the oldest item wins every
time until acted on; if the user is not going to act on it, it blocks its band
indefinitely. Random gives everything a turn.
- **Explicability holds.** "Why this one?" answers in one sentence: *these were
equally eligible, you get one* — arguably more honest than an ordering whose
inputs the user cannot see.

Stability is per user per day. Generalise the `random-stable-daily` idea from
content sources into the **general** in-band rule, so the companion has a
consistent voice for the day rather than changing on every refresh.

### Suggestion key

**`(source_id, entity_id, optional entity.prop(s))`**

The optional property component is what lets a condition be recognised as *new*
easily — e.g. a proposal going `draft → sent → draft` yields a different key, so
an old snooze does not suppress a genuinely new stall. Without it, the key is
stable across a condition resetting, which is wrong.

Used by: cooldown, snooze, and dedup across renders. For the entity-less case
(S6) the key degenerates to `(source_id)`.

### Affordances (RENAME — collides with `internal/affordances`)

One-key discriminated union, sorted by what they cost the user: `action:` /
`set:` / `pick_one:` / `navigate:` / `form:` / `snooze:` / `dismiss:` /
`acknowledge:`.

- `pick_one` earns its place because its options come from a query at render
time — a static button list cannot express it.
- Multi-step wizards are deliberately NOT affordances; a big form is a
destination a button points at. `Form.Steps` is the hand-off target. This keeps
the vocabulary small and needs nothing from interactive flows over HTTP.
- **dismiss/snooze are not optional.** Without "not now", the only way to clear
a suggestion is to comply — which makes it a demand rather than a hint. It is
also the best available signal: a source dismissed every time is a source to
delete.

### Muting: per-source, not per-entity

Per-entity mute was considered and **rejected**. It is invisible state — "never
suggest about Dana" lives somewhere nobody will look, and months later the
system is silently quiet about a third of the graph with no discoverable cause.

Per-source mute is discoverable by construction (5–20 named sources in config;
"which have I turned off?" is a short settings list), reversible obviously, and
yields an actionable operator signal — a source most users mute should be
deleted. Variable-duration snooze covers the per-entity case well enough, and
**expires**, so the failure mode is a suggestion reappearing rather than one
silently never appearing. For an advisory system that is the right direction to
fail.

### Per-user state — a new service with per-backend implementations

| State | Key | Nature |
|---|---|---|
| Snooze | suggestion key + `until` | explicit, temporary |
| Mute | `(user, source)` | explicit, reversible |
| Last-shown / cooldown | suggestion key | telemetry, disposable |

Snooze and cooldown key on the **suggestion key** above — `(source_id,
entity_id, optional entity.prop(s))`, scoped to the user. An earlier draft
carried a separate `dwell_start` on the snooze record to invalidate a snooze
when the condition reset; that is redundant, because the optional property
component of the key already does that job. A `draft → sent → draft` transition
yields a different key, so the old snooze simply does not match. One mechanism,
not two.

None is graph content. A snooze is not a fact about the entity; it is a fact
about one person's relationship to a suggestion at a moment. Putting it in the
graph makes it visible to everyone and audited forever — wrong on both counts.
Writes would also flood the append-only audit log and feed the postgres version
sweep. `least-recently-shown` in particular means writing on *render*, which is
the expensive one; date-seeded `random-stable-daily` avoids the log entirely and
is the better default for content sources.

**This is a new service with mem / disk / postgres backends, selected by build
tag — not a file-vs-store choice.** It follows the established pattern exactly:
one recipe per scenario (`appbuild_{fs,memory,postgres}.go`,
`mcp_wiring_{fs,memory,postgres}.go`) over shared `prepare()` / `assemble()`
helpers, with backend-specific imports confined to the tagged files so the
postgres build never links bleve and the default build never links pgx.

Consequences that follow from taking the pattern seriously — these, not the
storage medium, are the actual design work:

- **Backend-agnostic wiring belongs in `prepare`/`assemble`, never in a recipe.**
A recipe owns only the backend choice. Anything copy-pasted between recipes is a
shared helper. This is what stops the three from drifting.
- **A conformance suite is the deliverable, not an afterthought.** The store has
`internal/store/storetest` (`RunAll` + fuzz) precisely so a new implementation
must prove equivalence; user-state needs its own, with the postgres suite
DB-gated on `RELA_TEST_DATABASE_URL` like pgstore's. Three backends without a
shared contract test is three subtly different behaviours.
- **Expiry/pruning semantics must be identical across backends.** Snooze has a
TTL, so "is this snooze live?" has to answer the same on mem, disk and postgres
— including clock handling. Pin it in the conformance suite. The `today()`
UTC-truncation precedent (RR-YPYTP) is the cautionary tale: a local-vs-UTC
midnight skew of one day was a real bug.
- **The disk backend is app-written, TTL'd and prunable** — a different discipline
from `.rela/user-defaults.yaml`, which is hand-editable config. Same directory,
different file, and it should be gitignored like `.rela/scheduler-state.json`.
- **Multi-process concurrency is the postgres backend's problem to solve**, not
the interface's to expose. fs/mem are single-writer by nature; postgres has
multiple processes writing the same user's state. Keep the interface free of
locking primitives and let the recipe's backend honour the contract.
- **Constructors reject nil required fields** and never silently substitute a
no-op backend — a next-action layer that quietly stops remembering snoozes is
exactly the deferred-failure-to-downstream-symptom this rule exists to prevent.

## Cost model

Constraint: a properly configured system runs **1–10 fast queries**, and page
loads must not become unreasonably more expensive. No slow sources — all should
be graph-queryable and fast on the postgres backend.

- **Bound belongs to the engine, not config.** A per-source `limit:` is an
operator writing a number whose correct value depends on the other sources and
the page budget — not knowable at the site where it is written. Same failure as
per-source priority numbers.
- **A bound is not lossy here** because only one suggestion is ever shown. Sixty
stalled tasks and six produce identical output.
- **Short-circuit in band order.** Evaluate sources in band order, stop at the
first hit. A typical page runs one or two queries, not ten. The ambient source
at the bottom is reached only when everything above is empty — which is also
when it is cheapest to be there.
- **One query per source: yes** (`GraphQuery` as it stands). **One union query
across all sources: measure first.** A union defeats short-circuiting (runs all
ten every time) and couples sources at the SQL layer, breaking the
independent-deletability property that is the unit of iteration. `GraphCount`
already sets the precedent that two round trips can beat one clever statement.
Deferrable without cost — both forms sit behind the same source abstraction.
- **Degraded mode is a quip, not a spinner or a blank.** Exhausting a total
budget falls through to the ambient band. Only acceptable because the system is
advisory.
- **Resolve asynchronously**, so a slow high-band source does not block render.
A hint materialising a beat after the page reads as considered, not broken.

### No caching of resolved suggestions

**Decided: this system does not cache.** rela is ACL-gating throughout, and a
cache across principals is a footgun waiting to go off. The read path is gated
by `internal/visibility` decorators with row-level semantics where *a hidden
entity is nonexistent* — so a suggestion cache keyed on anything but the
principal would silently defeat that gate, and the failure mode is a user shown
a suggestion about an entity whose very **existence** is meant to be secret.
That is the strongest guarantee the security model makes, and the one most worth
not undermining for a page-load saving.

Per-principal caching would be safe but is not worth it: with short-circuit
evaluation a typical page runs one or two fast queries. Caching that saves
little and risks much.

The constraint this implies is the right one: **make the queries fast rather
than caching slow ones.** It reinforces "1–10 fast queries, no slow sources"
instead of working around it.

### Backend parity is the real constraint

Express sources in terms of `store.GraphQuery`, **not** SQL. The moment a source
expresses something only pgstore can answer, the default build (`rela`,
`rela-server` — most users) breaks or silently diverges. If a rule needs
something outside the model, extend `GraphQuery` in both implementations plus
the conformance suite. Expect constant pressure to "just add it in SQL, it's
only for the fast path" — resist it.

## Scenarios (grounding set)

| # | Scenario | Band | Needs | Store change |
|---|---|---|---|---|
| S6 | First run — "Nothing here yet. Start with a client?" | blocking | `GraphCount`, entity-less | **none** |
| S8 | Quip — ambient content, `pick: random-stable-daily` | ambient | stable date seed | **none** |
| S3 | "Meridian has no billing contact" | tidying | relation negation | negation |
| S2 | "40 minutes free — this is a small one" (`pick_one`) | tidying | property predicate | property |
| S4 | "SOW has been in draft a while. Send it?" | stalled | property + dwell | property + dates |
| S1 | "Proposal out 11 days, no reply. Chase it?" | blocking | property + dwell | property + dates |
| S5 | "Three completed projects have no invoice" | time-critical | see below | property + negation |
| S7 | "You haven't logged anything about Dana in 3 months" | ambient | far-side recency | **out of scope** |

Notes:

- **S1 vs S4** are near-identical in shape but opposite in ownership (ball in
their court vs. yours). Same machinery, different band. Good test that bands do
real work and that ordering is explicable in one sentence.
- **S2 and S8 are the counterweight.** S1/S3/S4/S5 are all, in their way, the
system reporting unfinished business. A configuration made only of those is a
nag however well-tuned. Ship at least one opportunity/content source in v1 so
the shape of a *good* configuration is visible from the start.
- **S7 is the deliberate out-of-scope marker** — obviously desirable, needs
far-side/recency predicates (a join with a filter on the far end). Recorded so
the exclusion is a decision, not an oversight.

## Decisions taken

1. **Set-shaped sources are OUT of v1.** S5 ("three unbilled projects") becomes
one source yielding three candidates, one shown. Acting on it reveals the next
tomorrow — slower than batch triage, but the system was never trying to be that.
Already helpful, not 100% perfect.

The killer argument is keying: `A+B` vs `A+B+C` has no principled equivalence
rule. Keyed on set identity it re-nags on every change; keyed on source identity
it never re-fires even when the set doubles. Both wrong in obvious cases, and
unpredictable to the user — failing the explicability bar the bands exist to
protect. Better no sets than unexplainable sets.

2. **Per-entity mute is OUT.** Per-source mute only. See rationale above.

3. **`GraphQuery` property predicates are the load-bearing gap.** They appear in
5 of 7 shapes. Options considered: extend `GraphQuery` (chosen), use
`SearchVisible` (documented-slow, no pushdown), or Go-side filtering over a
relation-narrowed set (degenerates to a full type scan when there is no relation
predicate).

4. **Relation negation is a small, independently useful addition** — the
`analyze_orphans`/cardinality family is negation-shaped. Likely in.

5. **Far-side predicates (S7) are OUT of v1.** This is where query languages go
to grow forever.

6. **Date arithmetic is out of scope** for this work (separate: pushing date
conditions into storage queries). Meets this design at the `GraphQuery`
predicate seam, which is a reason to keep that seam clean now.

7. **Analysis/validation as a source is possible but constrained.** Not a
replacement for the model — one candidate source family among many, and with a
different purpose (a report has an opinion about correctness; next-actions is
about what one *could* do). If bridged: consume cached/scheduled results only
(never live — Lua on the read path is forbidden), cooldown per `RuleName` not
just per entity (otherwise 40 firings are walked one page load at a time), and
rules **opt in** — the set of things worth reporting is much larger than the set
worth interrupting someone about.

8. **Empty-vs-absent is ALREADY DECIDED in `internal/filter`** — no new operator
needed. `internal/filter/match.go:31-41` documents the semantics: *"missing or
empty properties do NOT match any filter, except when explicitly checking for
empty values with `property=`"*, giving `property=` → "is empty" and
`property!=` → "is not empty". Missing and present-but-empty are already
collapsed into one empty-ish notion, which is exactly what a human means by "is
this field filled in?".

This is live and exercised, not dormant: the live `done-research-needs-summary`
validation rule uses `Then: ["summary!="]`. So an "incomplete profile" source
writes `prop:billing_email=` and needs nothing new.

Lua has no `is_empty` convention (and no `?`-suffix convention — that is
Ruby/Scheme); rela's bindings use plain snake_case verbs. A Lua-side `is_empty`
helper would be a small separate convenience — scripts currently hand-write `v
== nil or v == ""` and will sometimes forget a half — but it is out of scope
here. **If it ever lands it MUST mean exactly what `prop=` means in filter**;
two spellings of "empty" that disagree at the edges is worse than neither.

9. **Bands are operator-defined, and dwell time is OUT.** See the bands section
above. Ordered list in config, sources reference by id, stable-random within a
band. This is strictly *less* machinery than the original design: the fixed band
vocabulary and the dwell-time tiebreak both drop out.

Note this dissolves the S2-vs-S8 tiebreak question entirely. S2 ("40 minutes
free, here is a small one") is defensibly `tidying` rather than `ambient` —
small, optional, non-urgent — which leaves `ambient` holding only content and
removes the collision. With operator-defined bands, an operator who wants
opportunities separated from quips simply adds a band; the engine needs no
opinion about it.

10. **No caching** (see the cost-model section). A cache across principals is a
footgun against the ACL gate; per-principal caching is safe but not worth it
against one or two fast queries.

11. **Principal binding is DEFERRED and reframed** — not "principal binding in
the query language" but **permission-filtered candidates**. The profile case
models as *"select from profile where `<prop>` is empty AND the principal has
write-permission"*, letting the ACL gate identify "mine" rather than a new
`principal.id` binding. The system already computes per-entity write verdicts
including entity-scoped local roles (`alice --owner-of--> X`), so "my profile"
falls out of an existing capability.

This generalises better than `principal.id` would: *assigned to me*, *owned by
me*, *actionable by me* are all write-permission questions, and framing them
that way makes a suggestion **actionable by construction** — a source that
surfaces only what you can act on can never suggest something you are powerless
to do, which a naive id-match would happily produce.

The query does not exist yet. It is a third query-layer axis alongside property
predicates and negation, and the most delicate: it must be **consistent with the
real ACL evaluation, not a reimplementation of it** — two permission engines
that disagree is a security bug, not a discrepancy. Sits with date arithmetic as
a deferred query-layer extension.

12. **Operator instrumentation is a FOLLOW-UP: a separate analytics log +
service, NOT the audit log.**

The need is real — "use it with taste" is advice with no feedback unless
dismiss/mute rates are visible somewhere, and the dismiss signal is the whole
basis for "a source dismissed every time is a source to delete".

**It must not go into `internal/audit`.** That package "records every entity and
relation write" and is explicitly *forensic*; its `Op` constants are a
documented **"stable wire contract; downstream readers (jq, tail) match on these
literals"**. Adding suggestion telemetry would (a) break that contract for
existing readers, (b) dilute the `denied-write` / `acl-bypass` security trail
with high-volume UI noise — one user clicking through suggestions all day can
out-produce a week of real writes, and (c) reintroduce by another path exactly
the audit-log flooding that ruled out graph-backed suggestion state. There is
also a semantic mismatch: audit answers "what changed", and a dismiss changes
nothing — it is the user declining to act, a non-event.

Shape (recorded, not built):

- **The log** — its own append-only JSONL sink, borrowing the `audit.Filesystem`
  discipline and its `Nop`/`Memory`/`Filesystem` backend split. Log the
  **decisions** (`dismiss` / `snooze` / `mute` / `acted`), NOT impressions:
  "shown" is per-render and would dwarf everything else, and the signal wanted
  ("is this source annoying?") lives entirely in the responses. Last-shown is
  already tracked in the user-state service for cooldown, so impressions are not
  lost, merely unlogged.
- **The service** — whatever reads it (counts per source, dismiss rate, mute
  rate). Can honestly start as a documented `jq` one-liner and only become a
  service if that stops being enough.
- **Nop by default.** Unlike audit this is telemetry, not forensics, so an
  operator who declines it loses nothing. It is also per-user behavioural data —
  a mild privacy surface in a shared deployment — which is a further argument
  for opt-in, and for logging `(user, source, action, when)` and **not** the
  suggestion text.

Why follow-up rather than v1: an analytics schema designed before there is
anything to observe is guessing at its own dimensions, and those are sticky once
written. Phase 0 runs a handful of sources for a handful of users — a scale
where you learn by asking them, not by aggregating. Analytics earns its place
around Phase 2, when there are more sources than fit in one head.

The one piece with a "wish we had started collecting earlier" property is the
**log**, not the service; emitting it early (even with nothing consuming it)
means history exists by the time the service is wanted. That is the only part
worth pulling forward.

## Recommendation — build order

**Phase 0 (no store changes): S6 + S8, plus S3 if negation lands.** First-run,
ambient, and one real tidying suggestion. Enough to build bands, affordances,
the key, the user-state service (with its conformance suite across mem / disk /
postgres), and the one-slot UX against real output without touching the store
interface. Every hard question (does cooldown work, does one slot feel right,
does snooze expiry behave identically across backends) gets answered before the
expensive work starts.

**Phase 1: extend `store.GraphQuery` with property predicates** (both
implementations + conformance suite), and relation negation. This is the real
scope discovery — it makes a plain `type + property` source fast on postgres,
which is the most common source shape. Independently useful: the ACL gate,
dashboard cards, and `ViewTraverse.Where` would all benefit.

**Phase 2: the declarative config format** over both, once the model has held up
in one project.

This inverts the original request's suggested order (prototype-in-Lua first),
because a Lua prototype with hand-written predicates tests the wrong thing — it
proves Lua can do it, which was never in doubt, while leaving the actual
question (is the declarative form expressive enough?) unanswered.

## Open questions

None blocking Phase 0. The deferred items are all recorded as decisions above
with their rationale, and each is independently schedulable:

- Permission-filtered candidates (decision 11) — query-layer extension
- Date arithmetic (decision 6) — query-layer extension, separate scope
- Far-side / recency predicates (decision 5, scenario S7)
- Analytics log + service (decision 12) — follow-up, log worth emitting early
- Set-shaped sources (decision 1) — deliberately not planned

## Design principles to carry forward

- **Taste should be easy, not merely required.** Most annoying-companion failure
modes are preventable in the mechanism: one slot; conservative default cooldown
when unset (assume the operator got it wrong); one-click visible muting; and
eventually dismiss/mute rates as the operator's instrument (decision 12 —
follow-up). Until that lands, the feedback loop is asking users directly, which
is adequate at Phase 0 scale and not adequate beyond it.
- **Write down that it stays advisory.** The cooldown/snooze/shown-log machinery
is most of what a queue needs and there will be pressure to grow it. An advisory
system that becomes authoritative without anyone deciding to make it
authoritative is how you get a system people route around. The dismiss-signal
feedback loop only works while it stays advisory.
