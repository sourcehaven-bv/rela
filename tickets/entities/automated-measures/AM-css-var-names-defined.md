---
id: AM-css-var-names-defined
type: automated-measure
title: 'Test: every var() without a fallback names a defined custom property'
description: Pins BUG-PZIWJS. Scans .vue, .css and .ts sources in the SPA and the rela-components library and fails when var(--x) has no fallback and nothing defines --x. A fixture test pins the definition and use extractors, and a floor on scanned uses stops a broken pattern from passing vacuously.
kind: test
location: frontend/src/styles/cssCustomProperties.test.ts
status: active
---
