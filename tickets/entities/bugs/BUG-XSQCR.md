---
id: BUG-XSQCR
type: bug
title: Test hook window.__BACKTICK_AUTOCOMPLETE_DELAY_MS__ visible in production bundle
description: |-
    `MarkdownEditor.vue` read `window.__BACKTICK_AUTOCOMPLETE_DELAY_MS__` unconditionally on mount, so the test-only knob (which shrinks the inline-autocomplete open delay for e2e timing) and its magic property name shipped in the production bundle. An attacker with an XSS foothold could set it to force the popup on every backtick. No confidentiality/integrity impact — insertion still goes through `insertEntityRef` denylist validation — but test knobs should not be present in production bundles.

    **The catch:** the issue's recommended `import.meta.env.DEV` guard does not work here. The e2e suite runs against `bin/rela-server`, which embeds the frontend produced by `vite build` — and `import.meta.env.DEV` is `false` for **any** `vite build`, regardless of `--mode`. Guarding on it would strip the knob from the exact bundle e2e tests against, breaking `useFastAutocompleteDelay` (form.page.ts). DEV is tied to the vite *command* (serve vs build), not the mode.

    **Fix:** a compile-time `__E2E_TEST_HOOKS__` flag injected via vite `define`, true only when building in development mode (`vite build --mode development`, exposed as `npm run build:e2e`) and false for the default production `vite build`. The knob read is wrapped in `if (__E2E_TEST_HOOKS__)`, so it is tree-shaken (property name and all) from production bundles. A new `build-frontend-e2e` / `build-server-e2e` just target and the e2e recipes + CI E2E job build with `build:e2e`; the release build keeps the production `build`, so shipped binaries never contain the knob. vitest and eslint get the flag too (define / globals) so unit tests and lint don't break.

    **Trade-off:** the e2e suite now exercises a development-mode bundle (unminified, DEV warnings) rather than the exact production artifact — a mild, accepted fidelity reduction.
priority: low
effort: s
why1: MarkdownEditor.vue read window.__BACKTICK_AUTOCOMPLETE_DELAY_MS__ on every mount with no build guard, so the read shipped in production.
why2: The knob was added for e2e timing control (RR-1629) and guarding it was deferred; the magic global was the simplest way to pass a value into the component before mount.
why3: The obvious guard (import.meta.env.DEV) is unusable because the e2e suite runs the production-embedded bundle, where DEV is false — so a naive guard would break the very tests that need the knob.
why4: import.meta.env.DEV tracks the vite command (serve vs build), not --mode, so there was no built-in flag that distinguishes "e2e build" from "release build" — a project-specific compile-time flag was needed.
why5: There was no established convention for "test-only code that must compile into the e2e bundle but not the release bundle"; __E2E_TEST_HOOKS__ now provides that seam for future test hooks.
prevention: |-
    Verified by build: a production `npm run build` bundle contains no
    `__BACKTICK_AUTOCOMPLETE_DELAY_MS__` reference; the `npm run build:e2e`
    bundle does. The existing e2e spec
    `markdown-editor-backtick-autocomplete.spec.ts` (6 tests) exercises the
    knob via useFastAutocompleteDelay and passes against the e2e build,
    confirming the guard didn't break e2e.

    The `__E2E_TEST_HOOKS__` compile-time flag is the reusable seam for any
    future test-only hook that must reach the e2e bundle but not production —
    declared in vite.config.ts (define), vite-env.d.ts (type), vitest.config.ts
    (define, false), and eslint.config.js (global).
status: done
---

See GitHub issue #890.
