---
id: BUGA-B1MMGZ4
type: bug-analysis-checklist
title: 'Analysis: loadConfigPolicy drops the acl.yaml validation error silently'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (read of loadConfigPolicy on origin/develop: `_ = ValidateACLPolicy(p, meta)`)
- [x] Minimal reproduction steps documented (boot a flows project under acl.NopACL with an acl.yaml naming `user_entity_type: ghost`; it boots and logs nothing)
- [x] Environment/conditions noted (only hosts that inject an ACL without a policy: rela mcp, rela-desktop, read-only rela-server)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (keep booting, log the error with slog.Warn)
- [x] Regression test planned (the subtest "acl.yaml that does not fit the schema still boots" asserts the warning)
- [x] Related areas checked for similar issues (internal/cli/validate.go discards the same error, but `rela validate` reports it in its own acl.yaml check, so it is not silent)
