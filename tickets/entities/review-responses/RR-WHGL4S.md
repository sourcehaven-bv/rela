---
id: RR-WHGL4S
type: review-response
title: 'Test coverage: HTML render + injection escaping untested'
finding: gatherEnumHelp/mermaidStateDiagram/appDescription were tested but the hand-rolled HTML render (renderEnumHelp/renderHelpContent), the XSS/injection escaping of malicious values, the config wire-through, and the Vue components had no tests.
severity: minor
resolution: Added TestMermaidStateDiagram_InjectionSafe (space/arrow values + newline-label injection) and aboutDescription fallback test; updated TestMermaidStateDiagram for the aliased output. HelpModal/StatusBar component tests were not added — the behavior (mermaid render timing, token guard, About markdown) is verified live via puppeteer; a component test for the token-guard remains a defensible-to-defer gap. The renderEnumHelp HTML-string escaping is covered indirectly (values go through htmltemplate.HTMLEscapeString; prose through simpleMarkdownToHTML+DOMPurify at the sink, the established pattern).
status: addressed
---
