---
id: AM-settings-gating-and-readonly-piles
type: automated-measure
title: Settings gating, Settings and About builtins, and read-only pile controls
description: Go tests pin can_edit_settings on /_me per ACL mode and the settings and about builtins rows; vitest pins that a read-only workspace shows no pile change; the e2e settings-gating spec pins the Settings page per permission and the footer buttons.
kind: test
location: internal/dataentry/settings_authz_test.go, internal/dataentryconfig/builtins_test.go, frontend/src/components/piles/PilePanel.test.ts, e2e/tests/settings-gating.spec.ts
status: active
---

Fails when a user without settings:edit gets the shared Settings controls, when
a hidden Settings or About button shows, or when a read-only workspace offers a
pile change.
