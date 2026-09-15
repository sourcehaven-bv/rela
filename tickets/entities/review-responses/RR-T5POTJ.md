---
id: RR-T5POTJ
type: review-response
title: 'RenderStandaloneMarkdown must keep the command: assertion and the elevatedDeps call it inherits from RenderStandalone'
finding: 'Two things must move down into the new RenderStandaloneMarkdown when RenderStandalone is refactored onto it. (1) The fail-closed command: assertion. RenderStandalone, RenderListMarkdown and the plan all refuse a command: renderer, and those assertions live in the render layer precisely so the guard does not depend only on the call site; if the assertion stays in RenderStandalone while the Execute call moves down, RenderStandaloneMarkdown becomes an unguarded entry point. (2) The elevatedDeps(cfg) call (document.go:213-226), which supplies BOTH the elevation grant and cfg.Capabilities (TKT-YH52OM). renderScript:563, RenderListMarkdown:350 and RenderStandalone:407 all route through it. If the refactor hoists ExecuteStandaloneDocument without it, a standalone export silently loses elevation and declared capabilities. That fails closed, so it is not a vulnerability, but it is a confusing failure to diagnose. Also: the plan''s stated reason for refusing command: is only half right. For an ANCHORED document there is an entry entity, so renderCommand would work; the entity-less argument applies to standalone only. The real reason is the caching/asymmetry policy judgment, so the refusal is policy, not structural impossibility.'
severity: minor
resolution: 'RenderStandaloneMarkdown carries both the len(cfg.Command)>0 fail-closed assertion and the elevatedDeps(cfg) call; RenderStandalone is a thin wrapper adding markdownToHTML and keeps its signature, so its existing tests still pin it. The export handler additionally checks Command for the user-facing 400 (AC 5). Plan wording corrected: refusing command: is a policy choice, not a structural impossibility. TWO GAPS were found after this RR was first marked addressed, both predicted by it. (1) The anchored branch had no backstop below the handler, because RenderMarkdown legitimately dispatches to renderCommand for entity export; added the refusal in RenderDocumentMarkdown, above the kind dispatch so it covers both kinds, pinned by TestRenderDocumentMarkdown_RefusesCommandRenderer (verified to fail on the anchored subtest without it). (2) The promised elevation test had NOT been written - the code reviewer caught that this RR was marked addressed while every elevated assertion on the export route was a denial. Now written: TestExportDocument_ElevationAndCapabilitiesReachTheRender records the WriteDeps the engine actually received and asserts both ElevatedReader and the declared Capabilities survived. Verified to fail (both assertions) when elevatedDeps is swapped for luaDeps.'
status: addressed
---

## Resolution

1. `RenderStandaloneMarkdown` carries the `len(cfg.Command) > 0` assertion and
the `s.scriptEngine == nil || s.luaDeps == nil` check. `RenderStandalone`
becomes a thin `RenderStandaloneMarkdown` + `markdownToHTML` wrapper and keeps
its signature, so its existing tests still pin it.
2. The `elevatedDeps(cfg)` call moves down with the `ExecuteStandaloneDocument`
call. Add a test that a standalone export of an **elevated** document actually
receives the bypass binding and the declared capabilities — the failure is
silent-and-closed, so nothing else would catch it.
3. Also put an explicit `len(cfg.Command) > 0` refusal in the export handler
itself, for the clean 400 naming the limitation (AC 5). The render-layer
assertion is the backstop, not the user-facing error.

Correct the plan's wording: refusing `command:` is a **policy** choice (avoid an
anchored-only asymmetry, and the disk cache is HTML keyed on an entry hash), not
a structural impossibility. `RenderMarkdown` would happily run `renderCommand`
for an anchored document, which is exactly why the guard must not live only at
the call site.
