---
id: TKT-R8QEV3
type: ticket
title: Clear the 4 open go/path-injection and 1 js/xss-through-dom CodeQL alerts
kind: chore
priority: medium
effort: m
status: done
---

## Description

Five CodeQL alerts sat open on `develop` since April–June 2026: four
`go/path-injection` (`internal/storage/osfs.go`, `internal/storage/rooted.go`)
and one `js/xss-through-dom` (`frontend/src/utils/markdown.ts`). All four Go
alerts share a single taint source — the data-entry HTTP URL path in
`handleV1DynamicRoutes`.

They made the aggregate CodeQL check red on unrelated PRs, so they were worth
clearing on their own branch rather than folded into feature work.

### Were they real?

Mostly no, and that was verified rather than assumed. An audit of every
production holder of a raw `storage.FS`, and every direct `os.*` filesystem
call, found no reachable traversal: attachments, entity IDs, Lua, and the
data-entry handlers are each already gated. CodeQL flagged them because
`RootedFS.resolve` was a string-level validator returning a `string`, which the
analyser does not recognise as a barrier, and `mermaid.render()` is an opaque
third-party call.

One latent issue *was* found — see below.

### Changes

- `internal/storage`: `resolve()` returns a `ValidatedPath` (unexported field,
no exported constructor) instead of a `string`, and `RootedFS` reaches the
wrapped `FS` only through `validatedFS`. The documented "keys were validated"
convention becomes a compile-time property. A `contain()` chokepoint is the
single place a `ValidatedPath` is minted, turning "the segment rules are
exhaustive" into a checked postcondition.
- `internal/project`: `EntityTemplateVariantPath` now validates its own
segments. It joins into a raw `storage.FS` and automation interpolates
`{{new.kind}}` (an API-settable property) into the variant, so its only guard
living in `internal/automation` was one forgetful caller from a traversal. Not
exploitable today; the invariant is now local to the join.
- `frontend`: mermaid SVG is sanitized through DOMPurify before insertion.

### Notes for the next person

The obvious frontend fix is wrong. `DOMPurify.sanitize(svg, {USE_PROFILES: {svg:
true}})` strips `<foreignObject>`, which is where mermaid puts every flowchart
and state-diagram label — shapes and arrows still render, so it ships as empty
boxes. Sequence diagrams use SVG `<text>` and are unaffected, so checking one
diagram type hides it. `ADD_TAGS: ['foreignObject']` and omitting the `html`
profile are both load-bearing, and both are mutation-verified by tests.

Guard tests: `TestRootedFS_ReachesFSOnlyThroughValidatedFS` scans `rooted.go`
for raw-FS and direct-`os` calls (the style of `internal/acl`'s
`ceilingguard_test.go`), and `TestContain_*` pins the containment postcondition,
including the root-is-`/` case that a container or a tmpdir at the filesystem
root actually hits.
