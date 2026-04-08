---
id: RR-8PTK
type: review-response
title: generateDark TS port produces garbage hex for whitespace-only inputs
finding: |-
    frontend/src/utils/palette.ts:250 `generateDark` guards each field with `light.base ? adjustLightness(...) : ''`. JavaScript truthiness on `' '` (whitespace-only string) is `true`, so the guard doesn't fire. The value flows into `adjustLightness` → `hexToHSL` → `normalizeHex(' ')` → `' '.trim()` is in the older code path … actually `normalizeHex` does `raw.trim().replace(/^#/, '')`, so for a single space it becomes ''. Then it's neither length 3 nor 6, falls through unchanged, returns `'#'`. `parseInt('', 16)` returns `NaN`, the math propagates NaN, and `hslToHex` calls `Math.round(NaN * 255)` → `NaN` → `Number.toString(16)` → `'NaN'`, padded to `'NaN'`, joined to `'#NaNNaNNaN'`. That gets written into `paletteDarkColors[role]` and presented to the user as their derived dark value.

    Reproduce: in Light+Dark mode, type a space into the Light Accent text input, then click Derive Dark from Light. The Dark Accent input shows `#NaNNaNNaN`, which then fails the backend hex validation on save.

    The save side `normalizeColorInput` (SettingsView.vue:84) trims and ONLY normalizes if it matches `HEX_INPUT_RE`, otherwise stores the raw trimmed value. So a space becomes `''` on first input — but a partial hex like `#abc ` becomes `#aabbcc` (good), and `#ab` becomes `#ab` (stored verbatim, not a valid hex). Now click Derive: `light.accent = '#ab'` is truthy, `normalizeHex('#ab')` returns `'#ab'`, `parseInt('ab', 16) / 255 = 0.67`, the other two channels are NaN → `#abNaNNaN`.

    Fix: in `generateDark`, validate each input with the same `HEX_INPUT_RE` (or call `normalizeHex` and check the result is well-formed) before passing to `adjustLightness`. Treat invalid as empty. Also: lock this behavior with a unit test (`palette.test.ts` should have `it('returns empty string for invalid hex')` for both whitespace and partial input).
severity: significant
resolution: 'Added strict full-hex validation in `frontend/src/utils/palette.ts`. The new `isFullHex` predicate (uses `FULL_HEX_RE = /^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/`) is checked before passing any value to `adjustLightness`/`invertLightness`. Inputs that are empty, whitespace, partial hex, or non-hex are passed through as empty strings (light) or skipped (badges). Also added a helper that trims before testing so `''  #abc ''` works. New Vitest tests in `palette.test.ts` cover whitespace, partial hex, wrong length, non-hex, and 3-digit-with-padding cases — assert no NaN ever leaks into output.'
status: addressed
---
