---
id: DOCS-C09N2R
type: docs-checklist
title: 'Docs: ACL read-side: /_search visibility — VisibleSearcher seam, generic + pgstore-native impls, conformance suite'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on all new exported symbols: `search.TypeScope`, `search.WildcardType`, `search.ResolveTypeScope`, `search.VisibleSearcher` (full contract incl. lookup rule, post-visibility limit, per-backend ordering), `search.ErrScope`, `search.NewVisible`, `pgstore.SearchVisible` (SQL shape, ErrScope rationale, LIMIT rule), `storetest.VisibleSearchFactory`, `storetest.RunVisibleSearchTests`
- [x] readGate godoc extended with the SearchScope flavor + nop/ACL wildcard distinction
- [x] executeQuery godoc rewritten: gate ordering, cap model, error taxonomy, the two fixed silent failures

## Project Documentation

- [x] GUIDE-acl-security (docs-project): new "Global search (`/_search`, TKT-BA8BSX)" section — scope lookup rule, post-visibility limit contract, generic-vs-native split, bleve 10k candidate window + load-amplification note, deny short-circuit, serializer invariant, error semantics; `_search` removed from "What still leaks"; threat-model summary updated
- [x] docs/acl-security.md regenerated via `just docs`
- [x] CLAUDE.md tests section: new VisibleSearcher implementations must pass storetest.RunVisibleSearchTests

## External Docs

- [x] ~~docs/metamodel.md / cli-reference.md / data-entry.md / README.md~~ (N/A: no metamodel, CLI, or UI surface change — the SPA search view consumes the same response shape, just correctly filtered)
