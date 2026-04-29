---
id: RR-H5E9A
type: review-response
title: Orphaned chromedp e2e tests target deleted form.html selectors and aren't wired into CI
finding: 'internal/dataentry/e2e_test.go (gated by //go:build e2e) contains chromedp tests that navigate to /form/create_ticket/TKT-001 and probe selectors like body-editor that came from form.html. The build tag is invoked nowhere (grep -rn ''tags=e2e'' .github/ justfile is empty) so these tests have been silently rotting. Playwright suite at e2e/ is the actual CI surface. Decision needed: delete this Go-level chromedp suite, or keep TestE2E_LuaDocumentRenders (which exercises the Lua → goldmark → URL-rewriter chain and is still valid) and drop the form-specific tests.'
severity: significant
resolution: Deleted TestE2E_MarkdownEditorSave and TestE2E_FormFieldSubmit from internal/dataentry/e2e_test.go — both probed selectors (body-editor, button.btn-primary, /form/create_ticket/) that came from the deleted form.html. Kept TestE2E_LuaDocumentRenders, which exercises the Lua → goldmark → link-rewriter → SPA DocumentsPanel chain through valid Vue selectors (.documents-panel .document-body) and is still meaningful. Removed the unused fmt import. Verified by `go test -tags e2e -run NONEXISTENT -count=1 ./internal/dataentry/` compiling clean.
status: addressed
---
