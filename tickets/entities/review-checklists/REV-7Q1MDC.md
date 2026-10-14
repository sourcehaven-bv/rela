---
id: REV-7Q1MDC
type: review-checklist
title: 'Review: Lua rela.xml.decode and rela.feed.parse with hardened XML limits'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (CI on PR #1879: test jobs passed on c70cf7b; the follow-up commit is re-checked by CI before merge)
- [x] Lint clean (`just lint`) (CI Lint failed on c70cf7b with misspell + ST1023; both fixed in the follow-up commit)
- [x] Comment lint gate clean (`just comment-lint`) (CI Comment lint passed)
- [x] Coverage maintained (`just coverage-check`) (CI coverage passed; safexml 100%, feed 95%)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-0NEKQ9, RR-EXLB6F (significant, addressed); RR-FMHWYV,
RR-DNR0VH, RR-EHK7W7, RR-RG2HCV (minor, addressed); RR-K8S628 (nit, addressed);
RR-XITTL6 (nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: rela.xml.decode node shape (TestLuaXML_Decode).
2. PASS: hostile XML refused (TestDecode_Refused, TestLuaXML_DecodeErrors).
3. PASS: all four formats (TestParse_RSS2/RDF/Atom/JSONFeed; live go.dev, hnrss, jsonfeed.org feeds).
4. PASS: dates to RFC 3339 UTC, raw kept (TestParseDate, TestLuaFeed_Parse).
5. PASS: id fallback, nil when absent (TestParse_RSS2, TestLuaFeed_Parse).
6. PASS: available in reader runtimes (all Lua tests run in NewReader).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-OAWOHH

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR #1879)
