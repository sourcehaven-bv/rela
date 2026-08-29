---
id: TKT-TNPRV8
type: ticket
title: 'tenant: provision a new org — eager IdP webhook plus a single-flighted lazy backstop'
kind: enhancement
priority: medium
effort: l
tags: [security, needs-design]
status: backlog
---

## Description

TKT-TNT9RS resolves a verified `org_id` to a store and **fails closed** on an
org it does not know. This ticket is the other half: making an org *become*
known — creating its schema, migrating it, and adding it to the tenant map.

RES-D54281 specifies the shape and the reasoning; this ticket carries it out.

## Approach (from RES-D54281)

- **Eager**: Pratique posts `org.created` to the existing self-authenticating IdP
  webhook (`internal/dataentry/webhook.go`, which already carries `org_id` and
  authenticates with a separate `aud`). The handler creates the schema and runs
  `pgstore.Migrate`. This is the fast path: the tenant's first request finds a
  ready schema, and a failure is loud and early where retry is easy.
- **Lazy**: a verified-but-unknown org arriving on a request provisions on the
  spot, which makes a missed webhook self-healing.
- **Both.** Eager is the fast path, lazy is the backstop.
- **Single-flight the lazy path.** `pgstore.Migrate` is already concurrent-start
  safe — one transaction under `pg_advisory_xact_lock(key,
  hashtext(current_schema()))` — so a herd serializes rather than corrupts. A
  single-flight in front makes nine concurrent requests wait on one migration
  instead of queueing nine.
- **Return `202` plus a status the SPA polls.** Do not hold an HTTP request open
  across schema creation and migration: that turns a slow migration into a
  client timeout with a half-created tenant and no owner.

## The trust assumption must be written down, not assumed

This is the part of the ticket that is a security decision rather than
plumbing. **Provisioning on an unknown-but-verified org means anyone who can
mint an assertion can create schemas.**

That is acceptable *today* only because of a specific, currently-true property:
there is exactly one issuer, `aud` is pinned, Pratique guarantees `org_id` is
present on every verified assertion, and an orgless session never receives an
assertion at all. So "verified org never seen" means *new*, not *malicious*.

**It stops holding the day a second issuer exists.** Record the assumption
next to the code that depends on it, and make the lazy path something an
operator can turn off — a deployment that provisions only via the webhook
should be able to say so and have unknown orgs stay a hard denial.

Note also the rate-limit dimension: schema creation is expensive and
unbounded-in-principle. A cap on provisioning rate is not gold-plating here.

## Interaction with the resident-set bound

TKT-TNT9RS bounds the resident set because each open store costs ~17
connections (untuned pool + dedicated LISTEN connection + sweep). Provisioning
adds a second, different pressure: the number of *schemas* on a cluster, which
is bounded by disk and by how long `pgstore.Migrate` takes to run N times with
mixed-version states. These are separate ceilings and should be reasoned about
separately.

## Scope

**In scope**

- `org.created` webhook handling: create schema, migrate, register the tenant.
- The lazy backstop, single-flighted, operator-disableable.
- `202` + a pollable provisioning status.
- Whatever the tenant map needs to accept a write at runtime. TKT-TNT9RS chose
  an operator-authored config file **precisely because there was no runtime
  writer yet**; this ticket is that writer, and is therefore the right place to
  decide whether the `Resolver` implementation moves to a control-schema table.
  The interface should not need to change — that was the point of the seam.
- Strict schema-name derivation from `org_id` (`^[a-z][a-z0-9_]{0,30}$`), since
  a schema name reached from a claim is a trust boundary.

**Out of scope**

- Resolution and routing (TKT-TNT9RS).
- Erasure (TKT-TNERAS).
- Reconciliation against the IdP. RES-D54281 identifies the one legitimate
  feature request to Pratique — org enumeration or `org.created` event replay,
  as the backstop for missed webhooks. Track separately; note that the lazy
  path already covers the common case.

## Acceptance criteria

1. An `org.created` webhook creates and migrates a schema, and the org resolves
   afterwards without a restart.
2. The lazy path provisions an unknown verified org, is single-flighted (pinned
   by a concurrency test), and can be disabled by the operator — with unknown
   orgs then remaining a hard denial.
3. Provisioning returns `202` and a pollable status; no request is held open
   across migration.
4. The trust assumption is documented at the code that depends on it.
5. Schema names derived from a claim are validated before use.
