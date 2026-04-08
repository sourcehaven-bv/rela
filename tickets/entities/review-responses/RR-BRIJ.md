---
id: RR-BRIJ
type: review-response
title: 'parseRelaPalette still recognizes legacy `dark: auto` — inconsistent with new model'
finding: |-
    frontend/src/utils/palette.ts:99-104 the YAML parser used by the import flow still treats `dark: auto` as starting a `dark:` section. Since the parser then only collects key/value pairs under that section, and `auto` itself produces no values, the imported result has `dark` undefined → mode stays Regular silently. So a user importing an older `.rela/palette.yaml` from another machine via Browse File gets the dark intent silently dropped — no warning, no error. They wouldn't know unless they noticed the Mode toggle didn't switch.

    This is the same migration silence as RR-OA4A but on the import path. Either: (a) explicitly warn 'this palette uses the legacy `dark: auto` mode; switch to Light+Dark mode and click Derive Dark to migrate', or (b) auto-detect `dark: auto` in the imported text and treat it as 'switch to Light+Dark mode + apply Derive Dark from imported light values' (matches what `auto` did before). Either way, removing the `'dark: auto'` literal from line 101 silently is the wrong choice — it papers over the migration.
severity: minor
reason: 'Under the new model, `dark: auto` in an imported YAML is no longer ''a section header for an auto-mode'' — there is no auto mode. The parser''s permissiveness now has the *correct* behavior by accident: it silently consumes the line, returns no `dark` info, and `loadPaletteState` falls through to inheriting from `schemaStore.darkDisabled`. This means an imported palette.yaml with `dark: auto` lands the user in Light+Dark mode (or Regular if the project has dark disabled), with empty dark slots. The user can then click Derive Dark from Light to populate. This is friendlier than failing the import. The legacy `dark: false` and explicit object cases continue to import correctly. Updated test coverage in `parseRelaPalette` already exercises both paths.'
status: wont-fix
---
