---
id: TKT-TNPRV8
type: ticket
title: 'tenant: provision a new org — eager IdP webhook plus a single-flighted lazy backstop'
kind: enhancement
priority: medium
effort: l
tags:
    - security
    - needs-design
started: "2026-11-14"
completed: "2026-11-14"
status: done
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
across schema creation and migration: that turns a slow migration into a client
timeout with a half-created tenant and no owner.

## The trust assumption must be written down, not assumed

This is the part of the ticket that is a security decision rather than plumbing.
**Provisioning on an unknown-but-verified org means anyone who can mint an
assertion can create schemas.**

That is acceptable *today* only because of a specific, currently-true property:
there is exactly one issuer, `aud` is pinned, Pratique guarantees `org_id` is
present on every verified assertion, and an orgless session never receives an
assertion at all. So "verified org never seen" means *new*, not *malicious*.

**It stops holding the day a second issuer exists.** Record the assumption next
to the code that depends on it, and make the lazy path something an operator can
turn off — a deployment that provisions only via the webhook should be able to
say so and have unknown orgs stay a hard denial.

Note also the rate-limit dimension: schema creation is expensive and
unbounded-in-principle. A cap on provisioning rate is not gold-plating here.

## Interaction with the resident-set bound

TKT-TNT9RS bounds the resident set because each open store costs ~17 connections
(untuned pool + dedicated LISTEN connection + sweep). Provisioning adds a
second, different pressure: the number of *schemas* on a cluster, which is
bounded by disk and by how long `pgstore.Migrate` takes to run N times with
mixed-version states. These are separate ceilings and should be reasoned about
separately.

## Scope

**In scope**

- `org.created` webhook handling: create schema, migrate, register the tenant.
- The lazy backstop, single-flighted, operator-disableable.
- `202` + a pollable provisioning status.
- Whatever the tenant map needs to accept a write at runtime. TKT-TNT9RS chose
an operator-authored config file **precisely because there was no runtime writer
yet**; this ticket is that writer, and is therefore the right place to decide
whether the `Resolver` implementation moves to a control-schema table. The
interface should not need to change — that was the point of the seam.
- Strict schema-name derivation from `org_id` (`^[a-z][a-z0-9_]{0,30}$`), since
a schema name reached from a claim is a trust boundary.

**Out of scope**

- Resolution and routing (TKT-TNT9RS).
- Erasure (TKT-TNERAS).
- Reconciliation against the IdP. RES-D54281 identifies the one legitimate
feature request to Pratique — org enumeration or `org.created` event replay, as
the backstop for missed webhooks. Track separately; note that the lazy path
already covers the common case.

## Acceptance criteria

1. An `org.created` webhook creates and migrates a schema, and the org resolves
afterwards without a restart.
2. The lazy path provisions an unknown verified org, is single-flighted (pinned
by a concurrency test), and can be disabled by the operator — with unknown orgs
then remaining a hard denial.
3. Provisioning returns `202` and a pollable status; no request is held open
across migration.
4. The trust assumption is documented at the code that depends on it.
5. Schema names derived from a claim are validated before use.

## Design decisions

As implemented.

- **Where.** Provisioning lives in the tenant host (`internal/tenanthost`),
before `Acquire`, because no tenant app exists yet for a new org. The host takes
a consumer-side `tenanthost.Provisioner` (`Provision(ctx, orgID)`). The wiring
in `cmd/rela-server` adapts `tenant.Provisioner.Create` with `adopt=false`.
Only an existing tenant with status `active` counts as success; an existing
tenant in any other status is answered 200 without a create, because its state
belongs to status sync and the operator (review RR-4LU5ZUD). The CLI `rela-server tenant create` stays
as the operator fallback.
- **Which refusals provision.** The resolver now marks a valid org with no
control row with `tenant.ErrTenantAbsent`, and a provisioning row with
`tenant.ErrTenantProvisioning`. Only these two start a create. An org id that
`SchemaFor` refuses, an erasing tenant, a suspended one and a schema collision
never do. The host also runs `tenant.SchemaFor` before it starts a run, so a bad
org id never reaches `Create` (pinned by
`TestProvision_InvalidOrgNeverReachesProvisioner`).
- **Operator switch.** `-tenant-provision` (`RELA_TENANT_PROVISION`):
`off` (default, today's behaviour: unknown orgs are a 403, the webhook
acknowledges and drops), `webhook` (eager only; needs `-webhook-audience` and
`-webhook-action`, refused at startup otherwise), `lazy` (webhook plus the lazy
backstop; works without the webhook, which then is simply not the eager path).
The value is validated at startup.
- **Eager path.** A verified `organization.created` for an org without a
tenant (or with a half create) starts a run and waits up to 10 s: success 200;
still running, or another process holds the tenant lock, 202 (the IdP does not
retry; lazy or a later event finishes it); failure 502 so the IdP retries; over
the cap 503 with `Retry-After`. `organization.created` for an active tenant is
routed to the tenant as before and answers 200. Other events for unknown orgs
keep today's behaviour. `organization.deleted` suspends the tenant for the IdP
and never erases it; erasure is operator only (TKT-TNERAS).
- **Lazy path.** A request with a verified but unknown org starts a run and
gets 202 at once, with `Retry-After: 2` and `{"status":"provisioning"}`. No
request is held open across a migration. With `webhook` or `lazy`, a tenant in
status provisioning also gets 202 instead of 403. A failed run answers 503
`provisioning_failed` with `Retry-After: 30` for 30 s, then the next request
retries.
- **Status route.** `GET /api/v1/_tenant` answers 200 with
`{"status":"active"|"provisioning"|"suspended"}` for the caller's own verified
org only. It takes no org parameter. An unknown org gets 403 `unknown_tenant`,
unless `lazy` is on, which starts a run and answers `provisioning`.
- **Single flight.** Runs are keyed by org id; concurrent requests and
webhooks share one. A run uses its own context with a 5 minute timeout, so a
client disconnect does not abort a migration (pinned by
`TestProvision_LazySingleFlight` and `TestProvision_RunOutlivesRequest`). Host
shutdown cancels runs in progress; the next create finishes them.
- **Rate cap.** Process wide: `-tenant-provision-rate`
(`RELA_TENANT_PROVISION_RATE`, default 10 starts per minute, token bucket with a
burst of the same size) and at most 2 runs at once. Over the cap: 503 with
`Retry-After: 10` for requests and the webhook.
- **Trust assumption.** Written at `tenanthost.provisioning`: one issuer,
`aud` pinned (the webhook has its own), Pratique always sets `org_id` on
verified assertions, so a verified unknown org is new, not malicious. Anyone who
can mint an assertion can create schemas. It stops holding when a second issuer
is trusted, which is why the default is `off`.
- **Seeding is out of scope.** Configuration is shared by design
(RES-D54281), so no per-tenant config is seeded and a new tenant starts empty.
No seed-data feature was added.
- **Erased tenants stay erased.** Erase used to delete the control row, so
an erased org looked absent and lazy provisioning or a replayed
`organization.created` recreated it. Erase now leaves a tombstone: status
`erased` with `erased_at`, and no tenant data. The resolver reports it as
`ErrUnknownTenant` plus `ErrTenantErased`, never `ErrTenantAbsent`, so the host
refuses it with a plain 403 and acknowledges the webhook with a warning.
`Provisioner.Create` refuses it with `ErrTenantErased` unless
`CreateOptions.Recreate` is set; only `tenant create -recreate` sets it. The
status check is widened and `erased_at` added on the first lifecycle command, in
one transaction.
- **Explicit IdP status.** The operator API client reads only `active` and
`suspended`. An empty, missing or other status is `ErrUnavailable`, so the IdP
retries and nothing resumes. pratique's `orgJSON` always sends the status and
maps an empty one to `active` itself.
- **Operator API transport.** The operator API answers are not signed, so
`-tenant-idp-org-api` takes `https`, `unix`, or plain `http` only to `localhost`
or a loopback IP literal. Any other address stops startup.
- **Another server provisioning.** A run that ends with `ErrTenantInUse` is
remembered for 5 seconds. During that hold a request for the org answers 202
provisioning and a webhook 202, without starting a run or taking a start-rate
token.
- **Writes during provisioning (review RR-ARWVX6I).** Only GET and HEAD
get the 202 provisioning answer. Any other method gets 503 `tenant_provisioning`
with `Retry-After`, because the SPA reads 202 as success and nothing was
written.
- **Refusals that need an operator (review RR-4LU5ZUD).** A create refusal
that retrying cannot fix is marked `tenanthost.ErrNeedsOperator`. The host holds
it for 1 hour, answers the webhook 503 with `Retry-After` so the IdP keeps
retrying until an operator fixes it, and answers requests 503
`provisioning_needs_operator`. During any hold, `Retry-After` is the time left.
A tenant that stays in provisioning is logged with a rate-limited warning.
- **Not changed.** The server still refuses to start without the control
table, so the first tenant is created with `tenant create`. The SPA does not yet
poll the status route; the API contract is in place.

## Suspension

- **Thin events.** Pratique's `organization.updated` names only the org, so
the host reads the org's status from Pratique's operator API (`internal/idporg`,
HMAC-signed `GET /api/v1/orgs/{id}`). The address is `-tenant-idp-org-api`; the
secret is `$RELA_IDP_OPERATOR_SECRET` only.
- **Where.** In the tenant host, before `Acquire`, through the consumer-side
`tenanthost.StatusSyncer`. So it works for a suspended tenant, which every other
event finds refused with 503. The event is not passed on.
- **Who suspended.** The control table has `suspended_by` (`operator` or
`idp`). `ResumeForIDP` is a compare-and-set on `suspended` and `idp`, so the IdP
never undoes an operator suspension. `SuspendForIDP` keeps an existing cause.
`tenant suspend` sets `operator`; `tenant resume` clears it.
- **Upgrade.** The column is added on the first lifecycle command. The
resolver reads it through `to_jsonb`, so a server reads an old table.
- **Answers.** 200 when applied or nothing to do (also no tenant, or an org
the IdP does not know), 502 when Pratique or the database cannot answer, 503
with `Retry-After` for a tenant still being created.
- **Ordering.** Two `organization.updated` events for one org could race: one
reads active, the other reads suspended and suspends, and then the first
resumes. Syncs of one org now take turns in each process, and each reads the IdP
only once it holds the turn. Across processes, a resume that changed the status
reads the IdP again and suspends again if the org is suspended by then. Pinned
by `TestStatusSync_ConcurrentSyncsTakeTurns` and
`TestStatusSync_ResumeRechecksTheIdP`.
- **Erased tenants.** `SuspendForIDP` and `ResumeForIDP` leave an erased
tenant alone, and the event is answered 200.
- **Effect on open tenants.** After a suspension the host calls
`Registry.RevalidateOrg`, which closes the open tenant at once, as the next
request would after `tenant suspend`.
