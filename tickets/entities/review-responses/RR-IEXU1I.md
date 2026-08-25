---
id: RR-IEXU1I
type: review-response
title: versionSweeper naming, and StartVersionSweep has no stop counterpart
finding: Two design observations. (1) versionSweeper is named for the noun the method starts, but the type does not sweep — it can be told to start sweeping; versionSweepStarter is uglier and more accurate. (2) StartVersionSweep returns nothing and has no stop counterpart, so the wiring site has no handle to stop what it started. The store's own Close presumably handles it today, but a future backend could leak a goroutine per tenant in a schema-per-tenant deployment.
severity: nit
reason: Both deferred to TKT-L3FNEN rather than churned here. (1) is low stakes and renaming now would conflict with that ticket, which reshapes these signatures anyway. (2) is a genuine design question about the capability's contract, not about this refactor — the interface faithfully reflects the existing method, and inventing a stop semantic is out of scope for a mechanical widening. Noted so the lifecycle gap is considered when the types are promoted.
status: deferred
---
