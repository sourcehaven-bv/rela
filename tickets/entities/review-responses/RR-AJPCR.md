---
id: RR-AJPCR
type: review-response
title: dataentry captures luaWriteDeps at NewApp time, never refreshed on metamodel reload
finding: 'internal/dataentry/app.go:211-218, 275 — readDeps/writeDeps are built from the meta passed to NewApp and stored verbatim. rebuildState refreshes AppState.Meta, but app.luaWriteDeps.Meta still points at the original. An action script calling rela.list_entities after metamodel reload filters against stale metadata. Pre-existing behaviour (old luaServices captured the same way), but the refactor was a good moment to fix. Fix: either materialize deps at request time from an up-to-date source, or lift deps into AppState so it refreshes with reloads.'
severity: significant
resolution: Removed the captured luaWriteDeps field on dataentry.App. Added App.luaWriteDeps() method that materialises the deps bundle fresh per call, reading Meta from a.State() (up-to-date) and using the immutable Store/EntityManager/Tracer/Searcher/ProjectRoot. handleV1Action now calls a.luaWriteDeps() per request so metamodel reloads propagate to action scripts immediately.
status: addressed
---
