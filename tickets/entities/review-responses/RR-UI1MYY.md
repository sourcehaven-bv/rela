---
id: RR-UI1MYY
type: review-response
title: Leadership update relies on a side effect inside locked's fn
finding: 'internal/appbuild/realtime_worker.go:346-350 and 372. `w.leading = w.locked(..., func(){ ...; w.reconcile(ctx) }) && w.leading` is correct only because Go evaluates locked() (which assigns w.leading inside reconcile) before reading w.leading. It is easy to break when someone reorders the operands. Fix: have reconcile return its error and assign leading explicitly: `ok := w.locked(...); w.leading = ok && reconcileErr == nil`.'
severity: nit
resolution: The locked helper and the leading side effect are gone; leadership is the lifetime of the lead call, tracked with an atomic flag.
status: addressed
---
