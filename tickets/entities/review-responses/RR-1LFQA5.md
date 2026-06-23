---
id: RR-1LFQA5
type: review-response
title: pgstore-native VisibleSearcher unreachable through planned wiring
finding: 'pgstore.Open returns search.New(st, backend) — the generic *search.Service — as a plain search.Searcher; SearchBackend has no visibility method and buildPredicateSQL/escapeLike are unexported. ''postgres recipe wires native'' has nothing to wire: no native VisibleSearcher value flows from pgstore to the App, and the App cannot downcast. Plumbing (new appbuild slot, constructor param, nil-check) was unbudgeted and would force a day-one redesign of the second commit.'
severity: critical
resolution: 'Plan rev 2: pgstore-native impl is a method on *pgstore.Store (in-package access to buildPredicateSQL/escapeLike/sqlBuilder, nothing exported). New VisibleSearcher collaborator slot threaded appbuild → cmd wiring → dataentry.App: fs/memory recipes wire search.NewVisible(searcher, st); postgres recipe wires the *pgstore.Store itself. dataentry.NewApp validates non-nil per constructor rule.'
status: addressed
---
