---
id: RR-AGSR
type: review-response
title: mustNewACL takes *App where store.Store would suffice
finding: mustNewACL only needs the store. Passing the whole app couples test wiring to a fatter object than required. Take store.Store directly; call sites become mustNewACL(t, policy, app.store) — more honest about the dependency.
severity: nit
resolution: mustNewACL now takes store.Store directly instead of *App. All 6 call sites updated to pass app.store.
status: addressed
---
