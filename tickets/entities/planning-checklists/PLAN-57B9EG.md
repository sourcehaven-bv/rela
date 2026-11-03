---
id: PLAN-57B9EG
type: planning-checklist
title: 'Planning: Palette: UI kit shades, teal, distinct orange/yellow and operator colours'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** A style colour today is one of seven names, `orange` and `yellow`
share one colour, and an unknown name silently renders grey. A risk heat map
needs graded colours (green, yellow, orange, red, dark red) and operator
colours. The brief is `.ignored/gaps/G3-dashboards.md`, ticket 1 (approved).

Terms:
- **Kit**: the Atlas project management UI kit. It has ramps 50 to 900 for
grey, red, blue, green, yellow, teal and orange. It has no purple.
- **Bare hue**: a colour name without a shade (`red`). It keeps today's look:
a tinted tag with readable text, and a status dot.
- **Shade**: `hue/shade`, for example `red/700`. It renders as a filled colour
with an automatic text colour.
- **Operator colour**: a name defined under `palette.colors`.

**Scope:**
- In: colour name grammar and validation (`grey`/`gray`, seven kit hues,
`purple`, `hue/50..900`, operator names). Unknown names are a config error.
- In: kit ramps as `--rl-kit-<hue>-<shade>` tokens with a `-fg` token, in the
rela-components palette, light and mirrored dark.
- In: distinct `yellow` (new tag pair and status token) and new `teal`.
- In: `palette.colors: {name: {light, dark?}}` and `palette.dark.colors:
{hue/shade: hex}` (dark override of a kit shade).
- In: `styles:` values accept every name; badges, kanban column dots, relation
columns and list sections render them.
- In: `palette.badges` accepts `teal`; `yellow` no longer shares `orange`'s
token.
- In: in-tree configs that use `cyan`/`pink` (`tickets/data-entry.yaml`,
`internal/configedit/testdata`) move to valid names.
- In: docs in `docs-project/entities/guides/GUIDE-data-entry.md`.
- Out: the matrix card (ticket 2 uses this vocabulary), colour in calendars
(own `ValidCalendarColors` set), a colour picker in Settings, auto-assigned
colours for new enums (order unchanged so existing projects keep colours).

**Acceptance Criteria:**
1. `styles: {prio: {hoog: red/700}}` loads; the badge has kit red/700 as fill
and a text colour with contrast >= 4.5 in light and dark.
2. A bare hue renders exactly as before for `blue purple green gray red
orange`; `grey` equals `gray`.
3. `yellow` and `orange` render differently (badge and kanban dot).
4. `teal` is valid as bare hue and with shades.
5. `palette.colors: {risico-kritiek: {light: '#7a0010', dark: '#ff6b6b'}}`
makes `risico-kritiek` valid in `styles:`; without `dark` the light value is
used in dark mode.
6. In dark mode `red/700` shows kit red/200; `palette.dark.colors: {red/700:
'#aa0000'}` overrides it, and switching back to light restores red/700.
7. Config validation rejects `pink`, `red/750`, `purple/500`, a bad hex, an
operator name that collides with a kit or tone name, and a dark override of an
unknown shade, with a message that names the valid forms.
8. Every kit shade and its `-fg` pass 4.5:1 in both themes (test).
9. Docs describe the names, shades, operator colours and dark behaviour.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: the brief was researched and approved with the user.

**Existing Solutions:**
- `internal/dataentryconfig/palette.go` (TKT-PALKIT/TKT-ME8LEI): palette roles,
badge overrides, `rlTokens` mapping onto `--rl-*` tokens, and the rule that a
key set in the light map is restated in the dark map because
`uiStore.applyPalette` never removes inline properties.
- `frontend/src/utils/styleColors.ts` `STYLE_PALETTE`: the one table of style
classes; Badge.vue and kanban columns read it.
- `internal/dataentry/app.go` `buildStyleMap`: maps config colours to classes;
unknown names fall back to grey silently (to be replaced by validation).
- `frontend/src/utils/tokenContrast.test.ts`: AA pins over the generated tokens.
- `frontend/scripts/gen-app-tokens.mjs`: tokens.css is generated from the
library palette, `apps_tokens.css` is a byte copy.
- Kit hex values: recovered from the user's kit image (pixel-sampled and read).
- No library needed: WCAG contrast is already in `frontend/src/utils/contrast`
and Go has HSL helpers in palette.go.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Go (`internal/dataentryconfig`):
- New `colors.go`: `kitRamps` (hue -> shade -> hex), the hue set (`grey`,
alias `gray`, `purple` bare only), `ParseColor(name, operator map) (Color,
error)` returning kind (bare, shade, operator), and `Color.StyleClass()`:
`badge-<hue>` (grey -> `badge-gray`), `kit-<hue>-<shade>`, `custom-<name>`.
`ForegroundFor(bg)` picks white or grey/900 when one reaches 4.5:1, else black
(always reaches 4.5:1 when white does not).
- `PaletteConfig.Colors map[string]OperatorColor{Light, Dark}`;
`DarkPalette.Colors map[string]string` keyed by kit shade name.
- `ValidatePalette`: operator names match `^[a-z][a-z0-9-]*$`, do not equal a
hue, alias or tone (`success warning error info new`), light is required, hexes
are `#rgb` or `#rrggbb` (no alpha: a translucent fill makes the automatic text
colour unprovable); dark override keys must parse as kit shades and take the
same opaque hex. `palette.badges` accepts `grey` as an alias of `gray`, and
`teal`.
- `ResolvePalette`: emit `--rl-custom-<name>` and `-fg` in light and dark (dark
falls back to light); for each dark kit override emit the override and its fg in
dark, and restate the kit light value and fg in light (applyPalette rule). User
palette colours merge over project colours.
- `rlBadgeTokens`: `yellow` and `teal` get their own tokens; amber is orange
only. `rlDefaultDarkTokens` gains the new keys. `ValidBadgeNames` adds teal.
- `validateStyles`: every value must parse with `cfg.Palette.Colors`.

Go (`internal/dataentry/app.go`): `buildStyleMap` uses `ParseColor` +
`StyleClass` instead of `colorToCSSClass`.

Frontend:
- rela-components `tokens.css`/`dark.css`: kit ramps (70 `--rl-kit-*` + 70
`-fg`), dark mirrors 900<->50 with recomputed fg, in BOTH dark blocks of
`dark.css` (`.dark` and the `prefers-color-scheme` block apps use),
`--rl-tag-yellow-*`, `--rl-tag-teal-*`, `--rl-color-status-yellow`,
`--rl-color-status-teal`. Generated by a small script so values come from one
table; regenerate `frontend/src/styles/tokens.css` and `apps_tokens.css`.
- rela-components types: `DotColor = StatusColor | \`--rl-${string}\``.
`Section.color`, `Swimlane.color`, `RlSectionHeading`and`RlStatusDot`accept it;
a`--rl-`value paints`var(...)`inline.`StatusColor` itself is unchanged, so
RlTag, RlTimeline and RlStatusPill are untouched.
- `styleColors.ts`: `resolveStyle(cls)` returns `{badgeClass | badgeStyle,
dot}`; Badge.vue, KanbanView and useRelationColumns use it.
- Badge.vue: `badge--yellow` uses the yellow pair, new `badge--teal`; shaded and
operator colours use inline `background`/`color` from the vars.

**Alternatives rejected:**
- Server emits kit vars in the resolved palette always: 280 inline properties
on `<html>`, and the AA pin would live in Go instead of the token test.
- Extending `StatusColor` with `yellow`/`teal`: every StatusColor consumer
(tag, timeline, pill, avatar, calendar) would need new classes.
- `cyan` alias for teal: the kit has no cyan; keeping the vocabulary equal to
the kit is simpler to document. In-tree configs are migrated instead.

**Files to modify:** `internal/dataentryconfig/{colors.go,palette.go,
validate.go}`, `internal/dataentry/app.go`,
`internal/dataentry/apps_tokens.css`,
`frontend/packages/rela-components/src/{styles/tokens.css,styles/dark.css,
types/index.ts,components/common/RlStatusDot.vue,
components/layout/RlSectionHeading.vue}`, `frontend/src/styles/tokens.css`,
`frontend/src/utils/styleColors.ts`, `frontend/src/components/common/Badge.vue`,
`frontend/src/views/KanbanView.vue`,
`frontend/src/composables/useRelationColumns.ts`,
`frontend/src/utils/tokenContrast.test.ts`, `tickets/data-entry.yaml`,
`internal/configedit/testdata/data-entry.yaml`, guide source.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** colour names and hexes come from
`data-entry.yaml` (operator) and the user palette saved from Settings (`PUT`
palette, validated by `ValidatePalette`). Names are an allowlist (kit table) or
match `^[a-z][a-z0-9-]*$`; hexes match the existing regex. Both end up in CSS
custom property names/values set through `style.setProperty`, so the strict
grammar prevents CSS injection.

**Security-Sensitive Operations:** none. Configuration is not a secret.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1/4/5: table test of `ParseColor`/`StyleClass`; `buildStyleMap` test;
Badge.vue test renders inline vars for `kit-red-700` and `custom-x`.
- AC2/3: Badge test for each bare class; styleColors test that yellow and
orange dots differ.
- AC5/6: `ResolvePalette` tests for operator colours (dark fallback), dark
kit override and light restatement.
- AC7: `ValidatePalette` and `ValidateConfig` negative table tests.
- AC8: `tokenContrast.test.ts` over all kit pairs in both themes; Go test that
`kitRamps` equals the `--rl-kit-*` light values in `apps_tokens.css` and that
`ForegroundFor` reaches 4.5 for every kit shade and random colours.
- Integration: a dataentry handler test loads a config with shades and an
operator colour and checks `/_schema` styles and `/_config` palette output.
- Manual: `just dev` with a project using shades, screenshots in light and
dark of badges and kanban dots.

**Edge Cases:** `gray` vs `grey`; uppercase names (rejected, names are
lower-case); `red/050`; `purple/500` (rejected); operator name `red-700`
(allowed, distinct slug from kit `kit-red-700`); `#abc` short hex (accepted) and
`#rrggbbaa` (rejected for operator colours); dark override of an operator name
(rejected: use `dark:` on the colour).

**Negative Tests:** see AC7.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Projects using `cyan`/`pink` (several sibling projects) fail to load after
upgrade. Mitigation: clear error naming valid colours; brief requires rejection;
noted in the PR and docs.
- Token drift between Go kit table and CSS: parity test.
- Inline-property leak across themes: restatement rule plus a test.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** data-entry guide (palette and styles sections),
regenerated `docs/data-entry.md`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-7W7TM5 (alpha hex), RR-7J0JQ1 (both dark blocks),
RR-YZWLGS (badges grey/teal); all addressed in the plan above.
