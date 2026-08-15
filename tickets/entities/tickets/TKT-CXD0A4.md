---
id: TKT-CXD0A4
type: ticket
title: 'Next-action layer Phase 0: engine, bands, user-state service (no store changes)'
kind: enhancement
priority: medium
effort: l
status: review
---

## Goal

Prove the next-action model end-to-end **without touching the store interface**.
Phase 0 of [[FEAT-79DTF9]]; design in [[RES-09YLLL]].

The point is to answer the hard questions — does one slot feel right, does
cooldown work, does snooze expiry behave identically across backends — before
the expensive query-layer work starts.

## Scope

Three scenarios, chosen because they need no new store capability:

- **S6 — first run.** "Nothing here yet. Start with a client?" Uses
`GraphCount`, available today. Entity-less: the key degenerates to
`(source_id)`.
- **S8 — content/quip.** `pick: random-stable-daily` over a type, date-seeded so
a refresh cannot re-roll it. Needs no per-render state.
- **S3 — missing relation** ("no billing contact"), *only if* relation negation
lands cheaply. Otherwise defer to Phase 1.

## Deliverables

1. **Source resolution engine** — evaluate sources in band order, short-circuit
on first hit, engine-owned candidate bound (no per-source `limit:`),
stable-random selection within the winning band.
2. **Operator-defined bands** — ordered list in config; sources reference by id;
config validation rejects unknown references. Ship a default set in the starter
config.
3. **User-state service** — snooze / mute / cooldown, with mem / disk / postgres
backends selected by build tag, following the `appbuild_{fs,memory,postgres}.go`
recipe pattern over shared `prepare()`/`assemble()` helpers.
4. **Conformance suite for the user-state service** — mirroring
`internal/store/storetest`; postgres suite DB-gated on `RELA_TEST_DATABASE_URL`.
Expiry/TTL semantics pinned here.
5. **Render surface** — a `script:`-backed dashboard card
(`DashboardCard` is currently closed to `count|table|breakdown`);
`DocumentConfig` is the proven template.
6. **Affordances** — at minimum `action` / `navigate` / `snooze` / `dismiss` /
`acknowledge`. `pick_one` if S2 is pulled in.

## Constraints

- **No caching.** A cache across principals defeats the ACL gate, where a hidden
entity is nonexistent. Make queries fast instead.
- **Resolve asynchronously** so a slow high-band source never blocks render.
- **Rename the config key** — `affordances:` collides with
`internal/affordances` (the ACL verdict resolver). Pick `offers:` / `responses:`
/ `actions:`.
- **Per-user state never goes in the graph** — it would flood the audit log and
feed the postgres version sweep.
- **Constructors reject nil**; never silently substitute a no-op user-state
backend. A layer that quietly stops remembering snoozes is exactly the
deferred-failure symptom that rule prevents.

## Acceptance

- Three sources configured and rendering one suggestion at a time
- Snooze expiry verified identical across all three backends (watch the
UTC-truncation precedent, RR-YPYTP)
- Muting a source removes it and is reversible from a settings list
- `just arch-lint`, `just ci` green

## Out of scope

Property predicates, relation negation (unless trivial), date arithmetic,
permission-filtered candidates, set-shaped sources, analytics — all recorded as
decisions in the research.
