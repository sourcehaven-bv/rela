---
id: RR-BEYMCU
type: review-response
title: SetWorlds swaps the tracer without synchronization
finding: SetWorlds replaces a.tracer after construction; safe only at startup.
severity: minor
resolution: The SetWorlds godoc states that it rebuilds the base tracer; like the other Set* wiring calls it runs before the router serves.
status: addressed
---
