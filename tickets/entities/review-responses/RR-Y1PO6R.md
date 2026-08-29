---
id: RR-Y1PO6R
type: review-response
title: Five hardcoded ICONS.<name> template references make five config-facing names a compile-time SPA dependency the plan treats as freely renameable
finding: |-
    The plan describes `ICONS` as a generated artefact whose keys are "the public contract" for config authors, and the risk table treats renaming as a config-compatibility concern only (RR-OX9WFS: "name after what the glyph depicts").

    But `Sidebar.vue` dereferences five entries **directly as object properties**, bypassing `resolveIcon` entirely:

    - `Sidebar.vue:162` — `ICONS.search`
    - `Sidebar.vue:167` — `ICONS.warning`
    - `Sidebar.vue:235` — `ICONS.apps`
    - `Sidebar.vue:248` — `ICONS.settings`
    - `Sidebar.vue:256` — `ICONS.sun` / `ICONS.moon`

    These are the SPA's own chrome (the search box, the validation-warning link, the Apps section, the mobile footer, the theme toggle) — not author-configured icons. Consequences the plan does not address:

    **1. Renaming one of these six names silently breaks the SPA chrome, not just a config.** `ICONS.sun` on a generated `Record<string, Component>` is typed as `Component`, not `Component | undefined`, so TypeScript does NOT error when the key disappears. Vue renders `<component :is="undefined">` as nothing. The icon just vanishes — no build error, no test failure unless someone asserted that specific row.

    **2. The generator makes this worse, not better.** Today the names sit in a hand-written literal a few lines from nothing else. After generation they live in a ~150-entry generated file, where a curation pass reorganising categories can plausibly rename `warning` → `triangle-alert` or `apps` → `blocks` to satisfy the "name the glyph, not the use site" rule from RR-OX9WFS. That rule, applied naively, argues FOR renaming `apps` and `settings` — precisely the two most load-bearing keys.

    **3. The existing guard is weaker than it looks.** `icons.test.ts:50-67` asserts twelve names are `isKnownIcon`, which covers presence but not identity, and omits `document`, `calendar`, `clock`, `status`. The plan's AC 8 says "pin each to its component", which would fix identity — but only if it actually enumerates all sixteen, and the plan does not say the SPA-chrome subset is special.
resolution: |-
  Addressed in plan. The generator emits named chrome exports (`IconSearch`, `IconWarning`, `IconApps`, `IconSettings`, `IconSun`, `IconMoon`) which `Sidebar.vue` imports, so a rename breaks `npm run build` instead of blanking a glyph. `IconDef` gains `Chrome bool` and the generator fails if a chrome entry is missing. AC 8 now enumerates all 16 names and asserts component identity.
severity: significant
status: addressed
---

## Why this is significant rather than minor

The plan's risk table already identifies "names are a frozen contract" as
**Medium likelihood / High impact**, citing RR-OX9WFS where this exact mistake
was made once. What it misses is that six of those names have a *second*
consumer with *no* validation path — the config allowlist protects authors, and
nothing protects the SPA.

The failure mode is silent. A missing config name is a loud startup error; a
missing `ICONS.apps` is an invisible gap in the sidebar.

## Recommended fix

**1. Make the chrome dependency explicit and type-checked.** Have the generator
emit named exports alongside the map, and let `Sidebar.vue` import those:

```ts
// generated
export const IconSearch = Search
export const IconWarning = TriangleAlert
export const IconApps = Blocks
export const IconSettings = Settings
export const IconSun = Sun
export const IconMoon = Moon
```

A rename then breaks `npm run build` at the import site, which is the whole
point. Alternatively keep `ICONS.*` but type the map as `Record<string,
Component | undefined>` so the dereference is checked — noisier at every call
site, so named exports are preferred.

**2. Mark the six names as pinned in the canonical table.** Add a
`ChromeDependency bool` (or a comment convention) to `IconDef` for entries the
SPA references directly, and have the generator fail if one is missing. That
puts the constraint next to the data instead of in a reviewer's memory.

**3. Strengthen AC 8.** It must enumerate all sixteen existing names AND assert
component identity, not just presence — and call out the six chrome names as
non-renameable for a distinct reason from the config-compat one.
