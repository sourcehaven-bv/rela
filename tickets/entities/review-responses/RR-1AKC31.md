---
id: RR-1AKC31
type: review-response
title: Five of eleven chrome names guarded nothing; the real Go-side coupling (bare string literals in the handler) was entirely unguarded
finding: |-
    The `chromeNames` set claimed to cover "the entries the SPA itself references by name ... the search box, the Apps section, the theme toggle, **the kind-derived nav glyphs**". The last clause was false.

    `dashboard`, `list`, `kanban`, `calendar`, `document` are not referenced by the SPA at all — grep confirms `IconDashboard`, `IconList`, `IconKanban`, `IconCalendar`, `IconDocument` had **zero** importers. They are chosen by GO, as bare string literals at `views_handler.go:355-376`. The generated exports were dead code a tree-shaker drops, and the chrome check verified an export nobody imported.

    So the actual coupling — Go literal → table name — was unguarded in exactly the way the chrome mechanism was invented to prevent.

    Failure scenario: someone applies RR-OX9WFS and renames `dashboard` → `house`. Validate fails on the missing chrome name; they add `house` to chromeNames; regenerate; TypeScript compiles; all tests pass. But `views_handler.go:364` still emits `"dashboard"`, `resolveIcon` misses, and every dashboard nav entry silently renders the fallback glyph. `sidebar_icon_test.go` asserted against the same stale literal, so it would have agreed with the bug.
severity: significant
resolution: |-
    Split the one dishonest set into two honest ones, each guarding its own coupling direction:

    - `spaChromeNames` (6 entries) — names the SPA imports. Maps to the TypeScript export identifier, now written in the data rather than derived. Only these get a generated named export; a test asserts the server-derived names do NOT get one, since an unused export reads as protection while protecting nothing.
    - `DerivedNames` — the glyphs the SERVER picks per entry kind. Exported so `navEntryToSidebarItem` names them (`derived.List`) instead of repeating literals, making a rename a Go compile error.

    `Validate` requires both sets to exist in the table. Added `TestDerivedIconsAreValidNames`, which asserts every glyph the handler can emit is in `ValidIconNames` — asserting against the allowlist rather than a literal is what makes it catch the rename. Fixed the misleading doc comment.
status: addressed
---
