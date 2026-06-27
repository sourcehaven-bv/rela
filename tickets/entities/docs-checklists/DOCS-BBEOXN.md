---
id: DOCS-BBEOXN
type: docs-checklist
title: 'Documentation: ACL-bypass automation scripts (TKT-D8T148)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Public API documented — `luaBypassACL` / `newElevatedHandle` (the closure + invalidated handle), `Manager.Elevated()` / `elevated()` / `gated()` (the leak-proof seam), `authorizeAndAudit`'s bypass branch + `recordACLBypass`, and the `OpACLBypass` audit op all carry contract comments explaining the security properties.
- [x] Non-obvious WHY captured — comments cite TKT-D8T148, the go-architect leak finding (why gated() at cascade dispatch), and why the handle is an object-capability rather than a ctx flag.

## Project Documentation

- [x] `docs-project/entities/guides/GUIDE-lua-scripting.md` — added an "Elevated writes — rela.bypass_acl" section (closure API, allow_acl_bypass operator gate, the four safety properties: audited/no-leak/closure-scoped/can't-forge). Regenerated `docs/lua-scripting.md`.

## External Documentation

- [x] ~~User guide / tutorial~~ (N/A: Lua API for script authors; the guide is the right surface).
- [x] ~~Changelog~~ (N/A: no separate changelog; PR description carries the summary).
- [x] CLAUDE.md — no new project-rule entry warranted beyond the existing entitymanager spoofing-rule update (rela.principal); the bypass contract lives with the code + the security tests.

## Verification

- [x] Docs accurate against current code — the documented behavior matches the test suite (bypass+audit, leak test, escaped-handle, absent-without-flag).
- [x] `just docs-check` passes once the regenerated guide is committed.
