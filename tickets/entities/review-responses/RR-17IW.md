---
id: RR-17IW
type: review-response
title: ResolvePalette panics if Explicit is non-nil but caller built DarkMode by hand without setting it via decoder
finding: |-
    internal/dataentryconfig/palette.go:293 dereferences `*darkMode.Explicit` after the gate `if darkMode.IsDisabled()`. The gate only checks `Disabled`, so any path that lands at line 293 with `Explicit == nil` panics with nil pointer deref.

    Reachability today: the user/project selection logic (lines 274-280) only assigns into `darkMode` if `Disabled || Explicit != nil`, so a non-nil-Explicit invariant *currently* holds. But this is fragile: any future code that mutates `darkMode` (e.g. a third overlay layer, or a future automation) only has to forget to maintain this invariant and you get a runtime panic in the rendering path. This will also be defaulted to `nil` in fuzz inputs / property tests.

    Also: the JSON path through `UnmarshalJSON` for `null` (line 113) returns a zero-value DarkMode with both `Disabled=false` and `Explicit=nil`. If a caller passes that zero-value through a higher-level struct that doesn't go through `ResolvePalette`'s gating helpers, the same panic occurs.

    Fix: at line 286, add `if darkMode.Explicit == nil { result.DarkDisabled = true; return result }` before the dereference. This is two lines and removes the only panic path. Add a test: `ResolvePalette(&PaletteConfig{Dark: DarkMode{Explicit: nil, Disabled: false}}, nil)` should return DarkDisabled=true, not panic.

    Secondary concern: `dark.Explicit != nil && all-fields-empty` (e.g. `dark: {}`) currently means 'inherit everything from light', but the resolved Dark map will be byte-identical to Light. There's no way to express 'I want dark mode enabled but with the same colors as light' vs 'I haven't decided yet'. Worth documenting in the godoc on `ResolvePalette` (and probably enforcing in the load path that empty `dark: {}` means `dark: false`).
severity: significant
resolution: 'Added defensive check in `ResolvePalette` (palette.go): after the `darkMode.IsDisabled()` gate, also check `darkMode.Explicit == nil` and treat as DarkDisabled. This is unreachable through the current code paths but defends against future regressions and JSON `null` decoding which produces a zero-value DarkMode that the original gate missed. Added regression test `TestResolvePalette/zero-value DarkMode (from JSON null) is treated as disabled` that decodes `"dark":null` into a PaletteConfig and asserts ResolvePalette doesn''t panic.'
status: addressed
---
