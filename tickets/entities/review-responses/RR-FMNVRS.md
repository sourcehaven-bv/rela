---
id: RR-FMNVRS
type: review-response
title: Acceptance criterion 'no webfont ships' was untestable by construction
finding: Three tests asserted the removed _rela-editor.woff2 path 404s and that toolbar buttons draw inline
  SVG. None asserted the shipped artifacts contain no font. assetsInlineLimit is deliberately Number.MAX_SAFE_INTEGER
  so the bundle is self-contained; a font pulled in by any future dependency is therefore base64-inlined
  into the JS or CSS and never becomes a served path. It would ship, all three tests would stay green,
  and the only signal would be the artifact quietly growing.
severity: significant
resolution: Added assertNoWebfont() to the editor build, gating both emitted artifacts on @font-face,
  inlined font data URIs, and url() font references. Verified it fires by appending an @font-face to the
  theme and watching the build fail. Split into its own plugin after discovering that a second generateBundle
  hook on the same plugin object silently replaces the first, which had stopped the stylesheet being emitted
  at all.
status: addressed
---
