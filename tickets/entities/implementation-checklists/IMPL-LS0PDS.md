---
id: IMPL-LS0PDS
type: implementation-checklist
title: 'Implementation: Surface metamodel doc-fields in data-entry help: per-entity values+lifecycle, plus a global help icon showing the app description'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written (help_docfields_test.go: gatherEnumHelp, mermaidStateDiagram + injection-safe, aboutDescription)
- [x] Integration verified live via the demo (help modal + About in the browser)
- [x] Happy path implemented
- [x] Edge cases handled (plain enum, non-enum prop, terminal machine, empty description, injection-hostile values)
- [x] Error handling: help-load token guard; mermaid render skipped on error

## Manual Verification

- [x] Each acceptance criterion verified

**Verification Evidence (live, prototype demo via puppeteer):**
- AC1: help modal shows a Values section per enum property with per-value descriptions.
- AC2: Lifecycle section renders a mermaid state diagram (SVG) + a transitions table with Help; one diagram per machine field. Diagram displays real values (aliased ids hidden).
- AC3: plain/non-machine fields add no sections.
- AC4: toV1CustomType + TestToV1CustomType_OmitsDocFields untouched (green).
- AC5: `ⓘ About` button in the status bar shows the deployment description; hidden when empty. Uses the dedicated `about_description` wire field (metamodel fallback), NOT AppConfig.Description (SettingsView safe).
- AC6: Go tests — TestGatherEnumHelp, TestMermaidStateDiagram(_InjectionSafe), TestAboutDescription. AC7: verified live.

Backend: `go test ./internal/dataentry ./internal/apiwire` pass; `go build
./...` OK; golangci-lint 0 issues. Frontend: `npm run test:run` 1356 pass;
typecheck clean; eslint 0 errors on touched files.

## Quality

- [x] Follows patterns (server help-HTML render like existing Properties/Relations; DOMPurify at the sink; renderMarkdown for About)
- [x] DRY — reused renderMermaidDiagrams / renderMarkdown / simpleMarkdownToHTML
- [x] Security — mermaid source injection-hardened (synthetic-id aliasing + label flattening); no XSS (strict mode + DOMPurify); no new wire-contract on toV1CustomType
- [x] No silent failures
- [x] No debug code left behind

**Note:** code review (cranky) raised 3 significant + 2 minor findings, all
addressed in commit a5158bc4 (RR-0GNDA9, RR-Y0RL1L, RR-BOIZB2, RR-NRJI7S,
RR-WHGL4S).
