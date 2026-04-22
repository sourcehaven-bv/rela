---
id: TKT-0K5YH
type: ticket
title: 'Add documents: block + e2e coverage for documents panel'
kind: test
priority: low
status: backlog
---

The frontend DocumentsPanel has a well-defined contract (initial render,
live-update via SSE, cache badge, Refresh button) but no e2e coverage because
the inline e2e test project doesn't configure any `documents:` entries.

To enable coverage:

1. Add a `documents:` block to `e2e/tests/fixtures.ts` DATA_ENTRY_YAML with a command that prints entity properties (see `prototypes/data-entry/project/data-entry.yaml` for a reference `ticket_summary` document). Depending on `.rela/bin/rela` means re-introducing the setupRelaBin helper that RR-26RE6 removed.

2. Add `documents:` coverage to `e2e/tests/` — three scenarios, all previously drafted as skipped stubs (TKT-4Q2VI review round 2):
   - Document content updates when an entity is modified (SSE round-trip)
   - Cached badge shows when content comes from cache on re-render
   - Refresh button forces document re-render

The previous `document-live-update.spec.ts` stubs were deleted because their
bodies contained only comments; there was nothing for an unskipper to run.
