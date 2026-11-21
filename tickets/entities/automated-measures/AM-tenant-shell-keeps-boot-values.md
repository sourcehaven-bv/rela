---
id: AM-tenant-shell-keeps-boot-values
type: automated-measure
title: 'Test: the SPA keeps the shell language and app name until its config loads'
description: Guards against BUG-HZS0DGQ. The vitest writes the rela-language and rela-app-name metas, loads locale.ts fresh, and mounts useShellAppConfig over a store that has not loaded. It fails if the boot language is cleared before the config loads or if a reload brings the shell values back.
kind: test
location: frontend/src/composables/useShellAppConfig.test.ts
status: active
---
