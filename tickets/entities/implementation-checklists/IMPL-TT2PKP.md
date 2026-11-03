---
id: IMPL-TT2PKP
type: implementation-checklist
title: 'Implementation: Palette: UI kit shades, teal, distinct orange/yellow and operator colours'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (colors_test.go: ParseColor table, ForegroundFor AA on every kit shade and 5000 random colours, kit ramps pinned to apps_tokens.css light and mirrored dark, operator colours, dark overrides, validation negatives; styleColors.test.ts, Badge.test.ts, names.test.ts, tokenContrast.test.ts kit pairs)
- [x] Integration tests written (test full flow, not just units) (TestValidateConfig_StyleColors loads YAML through ValidateConfig; TestBuildStyleMap case for kit/teal/operator classes; dataentry package suite incl. golden parity)
- [x] Happy path implemented
- [x] Edge cases from planning handled (gray alias, purple without shades, alpha hex, reserved names, -fg clash, gray+grey, dark-only keys, dark override restated in light)
- [x] Error handling in place (errors surfaced, not swallowed) (config load fails with the style entry path; stored user palette validated on load)

## Test Quality

- [x] Using fixture builders or factories for test data (loadAndValidate/testMetamodel helpers)
- [x] No hardcoded values in assertions when object is in scope (kit values asserted from kitRamps and ForegroundFor)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built the SPA and rela-server, ran a copy of prototypes/data-entry/project with
`palette.colors.risk-critical {light #7a0010, dark #ff6b6b}` and styles `open:
blue/700, resolved: teal, critical: risk-critical, high: orange, medium: yellow,
low: teal/300`. `/_config` served `kit-blue-700`, `badge-teal`,
`custom-risk-critical`, `kit-teal-300`; the palette served
`--rl-custom-risk-critical` with `-fg` #ffffff (light) and #25272d (dark).
Headless Chrome screenshots: list badges in light and dark (shades mirror, the
operator colour switches value, orange and yellow distinct), priority board
column dots in dark. Config with `pink`, `red/750`, `purple/500` fails to load
with the entry path (test).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (one ParseColor used by validation and buildStyleMap; one styleSwatch used by Badge, kanban and relation columns)
- [x] No security issues introduced (allowlist grammar for names; opaque hex only; SPA re-checks class grammar before building a var() name; stored palette validated on load)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
