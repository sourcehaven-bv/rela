---
id: RR-6XTXAW
type: review-response
title: appbuild wiring details
finding: derivedSchemaReconciler requires SetUniqueSpecProvider; ErrReconcileBusy is pg's; the shared loader must be build-tagged; query-index Properties must be normalized.
severity: minor
resolution: sqlite recipe declares its own Reconcile-only interface; loader tagged postgres || sqlite; Properties sorted and deduped as in pg.
status: addressed
---
