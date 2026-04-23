---
id: RR-0GPDV
type: review-response
title: Stale e2e test asserts /form/ticket/ after prototype migration
finding: TestE2E_LuaDocumentRenders at internal/dataentry/e2e_test.go:290-298 asserts /form/ticket/ is present and that edit:// was rewritten. After migrating prototypes/data-entry/project/scripts/docs/category_report.lua to rela.url("/form/edit_ticket/" .. id), the script emits /form/edit_ticket/... — the first assertion will fail and the second is redundant. Didn't surface locally because the e2e chromedp run was cached or gated; will go red on a clean CI run.
severity: critical
resolution: 'Updated internal/dataentry/e2e_test.go TestE2E_LuaDocumentRenders: assertion now checks for /form/edit_ticket/ (matching the migrated prototype script) and requires return_to= on the rewritten link. Dropped the redundant edit:// leak check since the script no longer emits that scheme. Doc comment updated to describe the new chain.'
status: addressed
---
