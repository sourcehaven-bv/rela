---
id: RR-X06ZWR
type: review-response
title: 'Multi-tenancy unaddressed: startMailWorker in assemble mints one worker + SMTP path per tenant'
finding: SharedBase.Assemble is called once per store/tenant, so starting the worker in assemble creates a goroutine, a buffer and an SMTP dial path per tenant. In schema-per-tenant postgres deployments that is N workers and N concurrent connections to the same provider, which will hit provider rate limits and connection caps. CLAUDE.md's SharedBase invariants also require Services.Close to tear down only what it was assembled with and never anything shared — a per-tenant worker satisfies that literally, but a later shared connection pool would violate it. The plan does not mention multi-tenancy at all.
severity: significant
resolution: 'Plan now states the decision explicitly: one worker per assembled Services (per tenant), matching the startDataMigration GC-sweep precedent which is likewise per-assembled. Total concurrency is bounded per worker (single sequential sender, no fan-out), and the docs note that a deployment with many tenants against one provider should expect N connections. A shared sender is explicitly rejected for this ticket because it could not live in assemble without violating the Close invariant; it is noted as a consideration for IDEA-WIJ2H1 where a shared queue backend would change the picture.'
status: addressed
---
