---
id: BUGA-E2HCR7
type: bug-analysis-checklist
title: 'Analysis: Client baseline drops a role''s face grants when the role also holds "*"'
started: "2026-10-18"
completed: "2026-10-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (`TestClamp_WildcardKeepsFaceGrantsTheCeilingPermits`, `TestAuthorizeWrite_CeilingKeepsFaceGrant` fail on develop; Atlas `verify/mcp-baseline/verify.sh` fails with "agent lost beleid create")
- [x] Minimal reproduction steps documented (bug body)
- [x] Environment/conditions noted (any principal under a client baseline with an allowlist on a write verb, role list holding "*" and a face grant)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (allowlist replaces only the wildcard; other entries filtered by type as in the non-wildcard branch)
- [x] Regression test planned (clamp-level and AuthorizeWrite-level tables over create/update/delete)
- [x] Related areas checked for similar issues (read: face entries dominated by "*", harmless; update/delete: same bug; also an allowlist of "*" dropped face entries; deny ceilings, rename, convert, worlds, permissions unaffected)
