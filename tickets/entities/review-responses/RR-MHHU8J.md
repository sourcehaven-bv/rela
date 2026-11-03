---
id: RR-MHHU8J
type: review-response
title: SQLite unsafe names need a fallback
finding: A property name the SQLite builder cannot render has no path.
severity: minor
resolution: sqlitestore.Rollup falls back to store.RollupGeneric when the builder marks itself unsafe.
status: addressed
---
