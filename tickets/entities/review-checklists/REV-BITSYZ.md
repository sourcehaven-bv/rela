---
id: REV-BITSYZ
type: review-checklist
title: 'Review: Lua rela.tagged_entities: find entities by version tag'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test ./... green except cmd/rela-desktop, which needs the built SPA bundle and is unrelated; sqlite-tagged store/appbuild/lua/visibility/dataentry green; postgres-tagged store/appbuild/lua/visibility green against a local database)
- [x] Lint clean (`just lint`) (golangci-lint 0 issues on the touched packages for default, sqlite and postgres tags; plimsoll and arch-lint clean)
- [x] Comment lint gate clean (`just comment-lint`) (no unresolvable doc links)
- [x] Coverage maintained (`just coverage-check`) (package floors pass)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer both ran)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-3KSOAG, RR-HYKXJM, RR-AARFQE addressed)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** design: RR-0364LX, RR-LCH63A, RR-74R3I1, RR-J64DJR,
RR-P3FDUY. Security: RR-GNBENK. Code: RR-3KSOAG, RR-HYKXJM, RR-AARFQE,
RR-S3Z9UK, RR-K51FCX, RR-513I3U (addressed), RR-MYCY4Q (deferred to TKT-D055SV).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: rows carry id, face, addr, type, version, deleted (storetest ListsTypeFiltersAndOrdinals; appbuild TestLuaTaggedEntities on sqlite and postgres).
2. PASS: type filter (same tests).
3. PASS: deleted rows listed for a history:read holder and omitted otherwise (TestLuaTaggedEntities dave vs erin; visibility Gate table).
4. PASS: unreadable live rows omitted (same).
5. PASS: renamed entity under its current id; version equals VersionByTag (storetest rename cases and the randomized model test check every row against ListVersions and VersionByTag).
6. PASS: one store query and one header batch per call (TestTaggedEntities_Budget at 10 and 50 rows).
7. PASS: unsupported backends raise the history error (Lua ReaderWithoutListingRaises; visibility Unsupported).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-X2RG7Z

## Final Checks

- [x] Commit message explains the why, not just what (minimal commit message per user preference, carries the ticket id)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (run after the ticket is done; PR URL and CI status are recorded in GitHub, see TKT-UFV01M)
