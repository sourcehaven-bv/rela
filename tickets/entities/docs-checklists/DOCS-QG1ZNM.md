---
id: DOCS-QG1ZNM
type: docs-checklist
title: 'Docs: Follow a retired id to its successor on the entity detail page'
status: done
---

## Code Documentation

- [x] Doc comments on new functions: redirectRetired, retiredLocation, retiredRedirectID, readAddressedOr, gateReadOr, viewsHandler.gateEntry/viewMiss/writeViewError, visibility.Resolver.Successor
- [x] Comments explain why: miss-only, never on writes, 307 + no-store, gated successor, log-and-miss on lookup failure, followOwner in the SPA

## Project Documentation

- [x] GUIDE-data-entry "Mentions after a rename" describes the redirect: when it applies, 307 with Cache-Control no-store, relative Location keeping the query, same answer as a missing id for a reader who cannot read the successor, writes not redirected
- [x] docs/data-entry.md regenerated with `just docs`

## External Documentation

- [x] ~~API reference~~ (N/A: the routes and bodies are unchanged; the redirect is described in the data-entry guide next to the retired-id behaviour it extends)
