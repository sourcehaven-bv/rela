---
id: RR-0PTF
type: review-response
title: yaml.v3 panics on duplicated-key shadow at unmarshal
finding: theme.go:18-27 + theme_test.go:92-127 — yaml.v3 *panics* on duplicated key tags during Unmarshal (verified empirically). The current TestThemeManifest_YAMLRoundTrip catches collisions on marshal output but not on unmarshal-time panics, leaving a runtime-fatal path live. Add a defer-recover in parseThemePackage that maps the panic to errInvalidManifest, plus a reflection-based test that fails at compile/test time when a yaml-tag shadow is introduced between ThemeManifest and PaletteConfig (so the test catches the bug before it ships).
severity: significant
resolution: 'Two layers: (1) parseThemePackage now defer-recovers and translates any panic into errInvalidManifest. (2) New init() in dataentryconfig/theme.go runs checkManifestTagsUnique() at startup via reflection — if any future PaletteConfig field shadows a top-level manifest key (yaml or otherwise), the binary refuses to start. Test TestCheckManifestTagsUnique pins the contract.'
status: addressed
---
