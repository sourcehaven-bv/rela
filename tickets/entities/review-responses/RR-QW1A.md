---
id: RR-QW1A
type: review-response
title: buildPalettePayload casts via `Record<string, string>` defeating the type system
finding: |-
    frontend/src/views/SettingsView.palette.ts:38 `(palette as Record<string, string>)[key] = val` casts away the typed `PaletteConfig` interface so any string key can be assigned. This is the only way to do dynamic key assignment without listing the eight role names, but it means typos in `state.light` keys (e.g. `acent` instead of `accent`) will silently end up on the wire and the backend's `accent` will fall back to default. The TypeScript compiler can't help.

    Fix: define a `const ROLE_KEYS = ['base', 'surface', ...] as const` and `type RoleKey = typeof ROLE_KEYS[number]`, then iterate over `ROLE_KEYS` instead of `Object.entries(state.light)`. The map then has compile-time guarantees that the keys are valid:
    ```ts
    for (const key of ROLE_KEYS) {
      const v = state.light[key]
      if (v) palette[key] = v
    }
    ```
    No cast needed, no string typo bug, and the same constant feeds `loadPaletteState` (currently the caller passes `paletteRoles.map(r => r.key)` from SettingsView.vue — three different definitions of the same eight strings). Consolidate them.
severity: minor
resolution: Added `PALETTE_ROLE_KEYS` const literal-typed array (`as const satisfies readonly (keyof PaletteColors)[]`) and `PaletteRoleKey` type in `SettingsView.palette.ts`. `buildPalettePayload` now iterates `PALETTE_ROLE_KEYS` instead of `Object.entries(state.light)`, so any typo in a role key fails at compile time and stray keys cannot be smuggled into the wire payload. The `Record<string, string>` cast was removed. Existing tests still pass.
status: addressed
---
