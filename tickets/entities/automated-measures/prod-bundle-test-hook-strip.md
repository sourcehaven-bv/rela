---
id: prod-bundle-test-hook-strip
type: automated-measure
title: 'Test: test-only hooks are stripped from production bundles'
description: 'Guards BUG-XSQCR. The __E2E_TEST_HOOKS__ vite define gates test-only knobs (e.g. the backtick-autocomplete delay) so they tree-shake out of the production `npm run build` bundle while compiling into the `npm run build:e2e` (development-mode) bundle the e2e suite embeds. The e2e spec markdown-editor-backtick-autocomplete.spec.ts (6 tests) exercises the knob via useFastAutocompleteDelay and passes against the e2e build, proving the production guard does not break e2e.'
kind: ci
location: frontend/vite.config.ts (__E2E_TEST_HOOKS__ define) + frontend/package.json (build vs build:e2e) + e2e/tests/markdown-editor-backtick-autocomplete.spec.ts
status: active
---
