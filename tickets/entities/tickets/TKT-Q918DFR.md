---
id: TKT-Q918DFR
type: ticket
title: Content-free keyset pages for search with no text
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

TKT-D9UO9Y4 made a search with no text read keyset pages
(`internal/search/index.go#Service.typePages`) and stop at the limit. Each page
is a `store.ListEntitiesPage` call, which returns full entities, so the page
reads bodies it does not use: a hit needs id, type, face, title and properties.
`.claude/rules/collection-reads.md` says collection reads are content-free, as
`loadFaces` in the same file already is.

Add a keyset-paged header read (a page method on `store.HeaderReader`, or an
after-key on `GraphQueryHeaders`), use it in `typePages`, and pin the change
with a body-read budget (`storetest.BodyWatch`). The old `listAll` read bodies
too, so this is not a regression.
