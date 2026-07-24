---
id: TKT-P1M7C
type: ticket
title: Secrets/settings system for Lua scripts
kind: enhancement
priority: medium
effort: s
status: wont-fix
---

> **Closed by backlog sweep (2026-07-20):** already implemented — `internal/secrets` loads .rela/secrets.yaml with global + per-script overrides; Lua exposes rela.secrets via WithSecrets (runtime.go) with tests.

Allow Lua scripts to access secrets (API keys, tokens) via rela.secrets table,
loaded from .rela/secrets.yaml with global and per-script overrides.
