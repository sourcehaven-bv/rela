---
id: BUG-DJZTRF
type: bug
title: Document view flashes empty state and scrolls to top on any unrelated entity write
description: DocumentView re-renders on every entity:changed SSE event regardless of type, and blanks docContent before fetching, so the view drops to the "No document content available" empty state and the browser resets scroll to top - even when the re-rendered HTML is byte-identical.
priority: medium
effort: s
why1: loadDocument() sets docContent.value = '' before awaiting the fetch, so during the in-flight re-render the three-way template chain falls past the delay-gated spinner branch into the v-else empty state; the tall content element leaves the layout and the browser clamps scroll to top.
why2: The re-render is triggered at all because handleEntityChange() calls loadDocument(true) on every entity:changed event without comparing the event's type against the viewed document, and the render returns byte-identical HTML that is then reassigned unconditionally, remounting the v-html subtree.
why3: The blanking line predates the anti-flash machinery. It dates to the original SPA migration (ebbcb5b6), when a synchronous blank-then-load was the whole loading story. The useDelayedPending gate (TKT-TFSNBY) was later added AROUND it, and its comment asserts 'a re-render keeps the previous document on screen rather than blanking it' - but the line it needed to delete was left in place, so the gate only suppresses the spinner while the blanking it was meant to prevent still happens.
why4: The gate's contract was expressed only as a prose comment, with no test observing the transient state during an in-flight re-render. Every available assertion (on docContent, or on the settled DOM after awaiting) is identical before and after the fix, because the defect exists only while the fetch is pending. The regression was therefore invisible to the test suite by construction.
why5: The SSE feed is a coarse, type-only staleness signal (TKT-POT9GQ) that every document view responds to with a full unconditional re-render. Because correctness never depended on the re-render being rare or cheap, no pressure existed to make it idempotent - so an expensive, redundant, visibly destructive refresh became the normal steady-state behaviour rather than an exceptional path, and the same copy-pasted loader shape reproduced it in DocumentsPanel.vue.
prevention: Pin the anti-flash contract with component tests that observe the INTERMEDIATE render (empty state never appears mid-fetch; .document-body DOM node identity survives an unchanged re-render) rather than the settled state - see AM-document-rerender-preserves-content. Make the idempotent path the default in both document loaders so a redundant SSE-driven refresh is a no-op by construction.
status: review
---

## Symptom

An open document in the data-entry SPA periodically flashes and jumps back to
the top of the page. The document content itself has not changed; the
re-rendered HTML is usually byte-identical. Any entity write anywhere in the
system triggers it, including writes from a scheduled Lua script, an external
file edit picked up by the fs watcher, or (on the postgres backend) a write from
another process.

## Reproduction

1. Open a document in the data-entry SPA and scroll down.
2. Write any entity of any type from anywhere (CLI, MCP, another browser tab,
an external edit to a markdown file).
3. The document flashes the empty state and scroll resets to top.

Nothing about the viewed document needs to change.

## Causal chain

1. Any entity write of any type broadcasts `entity:changed {type}` to every
connected browser (`internal/dataentry/watcher.go:229`). The frame carries a
type only, no entity id (TKT-POT9GQ).
2. `handleEntityChange` re-renders unconditionally, without even comparing the
event's `type` against the viewed document
(`frontend/src/views/DocumentView.vue:138-140`).
3. It passes `refresh=true`, which bypasses the server's content-hash disk cache
(`internal/dataentry/api_v1.go:2631`), forcing a full Lua/command re-render per
connected client per unrelated write.
4. `loadDocument` sets `docContent.value = ''` *before* awaiting the fetch
(`frontend/src/views/DocumentView.vue:100`).
5. The template is a three-way chain (`DocumentView.vue:180-193`). With
`docContent` empty and `showBlockLoader` still false (its 250ms delay gate has
not fired), the view falls through to the `v-else` **empty-state** branch and
renders "No document content available". The tall content element leaves the
flow, the browser clamps scroll to top, and the new HTML mounts at scroll 0.
6. The response carries no ETag and sets `Cache-Control: no-cache, no-store`
(`internal/dataentry/api_v1.go:2372-2377`). `ContentHash` exists server-side
(`internal/dataentry/document.go:577-590`) but is dropped from
`v1.DocumentResponse`, so neither side can detect the output was unchanged.

## Affected files

The defect is duplicated in **two** loaders with the same copy-pasted shape.
Both must be fixed, or the bug simply persists on the entity page:

- `frontend/src/views/DocumentView.vue` - blanking at :100 and :122, SSE handler
at :138-140, template chain at :180-193.
- `frontend/src/components/entity/DocumentsPanel.vue` - blanking at :124 and
:148, SSE handler at :158-160, template chain at :201-216. Carries the same
`showBlockLoader` gate (:42) and the same comment (:39-41) asserting the
behaviour the code does not implement.

## Scope of this bug

Fixes steps 4 and 5 (the user-visible flash and scroll reset) and adds an
identical-HTML guard so an unchanged re-render performs no DOM write at all.

Step 3 (not re-rendering for irrelevant changes) is deliberately **out of
scope**. Filtering would need to know which entities a document actually depends
on, and a `script:` document is arbitrary Lua that can read anything, including
conditionally, so the dependency set is not statically knowable from the config.
`entity_ids` on the response only ever carries the entry entity
(`computeDocumentHash` hashes `GetEntity(entryID)` alone), so it cannot support
the filter today. The same gap blocks a server-side ETag/304: the existing hash
covers the entry entity only, so a script document reading other entities would
report "unchanged" while its output genuinely changed. Both need the dependency
set widened first - separate work, and the reason this fix is client-side only.

## Note

`frontend/src/views/DocumentView.vue:37-39` already documents the intended
behaviour ("a re-render keeps the previous document on screen rather than
blanking it"). The `showBlockLoader` anti-flash gate was added around line 100
without removing it, so the stated intent was never actually implemented.
