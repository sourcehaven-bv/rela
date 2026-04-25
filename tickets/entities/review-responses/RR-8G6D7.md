---
id: RR-8G6D7
type: review-response
title: docs/metamodel.md mentions 'dedicated rename flow' without a link
finding: docs/metamodel.md:329-330 says 'the edit form shows the ID as a read-only display; renaming uses the dedicated rename flow' — no link, leaving the reader to wonder what the rename flow is.
severity: nit
reason: The rename flow doc target doesn't have a stable URL yet (FEAT-016 is the rename feature ticket; its surfaced doc page hasn't been written). Adding a hanging link to a non-existent doc would create a broken-link burden. Once FEAT-016 ships docs, this paragraph can be backfilled with a real anchor.
status: deferred
---
