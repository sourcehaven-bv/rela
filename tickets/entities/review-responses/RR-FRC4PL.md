---
id: RR-FRC4PL
type: review-response
title: 'Palette path dropped all three ring tokens — palette-configured custom apps rendered NO focus ring at all'
finding: 'The token contract has TWO renderers and only one is obvious. `appCSSSource` serves the embedded `apps_tokens.css` by default, but when a project configures a palette it REPLACES the whole `:root` block with `dataentryconfig.deriveTheme`''s hand-maintained map. That map had no ring tokens, so for any palette-configured project `_rela.css` left `--focus-ring`, `--error-ring` and `--focus-ring-gap` undefined and every `box-shadow: 0 0 0 2px var(--focus-ring-gap), 0 0 0 4px var(--focus-ring)` resolved to nothing — rendering NO ring whatsoever. That is worse than the bug the ticket set out to fix: the hardcoded indigo at least drew something. No test saw it: TestAppCSSSource only exercises the nil path, and TestAppCSSSourceUsesResolvedPalette asserts specific colors rather than completeness.'
severity: critical
resolution: 'Added the three role aliases to `deriveTheme`. They are `var()` references rather than derived colors, so they resolve against whichever palette produced the block and need no derivation. Added `TestPaletteCarriesEveryDefaultToken`, which parses both renderers'' `:root` output and asserts the palette path defines every token name the default path does — the SET, not any particular name, so it catches the next token too without needing an update.'
status: addressed
---

Confirmed by execution before fixing, not taken on the reviewer's word:

```
--accent-color       present=true
--focus-ring         present=false
--error-ring         present=false
--focus-ring-gap     present=false
```

Mutation-verified: removing the aliases fails the new test with
`palette path drops 3 token(s) the default path defines: [--error-ring
--focus-ring --focus-ring-gap]` and a pointer to `deriveTheme`.

**The generalisable lesson**, recorded at `deriveTheme` so the next person
meets it: this is the structural hazard of aliasing into a contract that has a
second, hand-maintained renderer. `TestAppTokensCSSInSyncWithFrontend` pins the
SPA copy against the Go copy byte-for-byte and gave real confidence — but it
only covers the *default* path, so it was green throughout.

`TestResolvePalette`'s "21 variables" assertion also had to move to 24. That is
a legitimate count change (8 base + 6 derived + 7 badges + 3 aliases), not a
test weakened to fit.
