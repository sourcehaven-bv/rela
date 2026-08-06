---
id: RR-AKQRNB
type: review-response
title: 'Sanitization test comment omits the javascript: href gap in happy-dom'
finding: 'The AC3 test comment in KanbanView.test.ts documents that happy-dom leaves onerror= on an <img>, but the reviewer verified happy-dom ALSO fails to neutralize javascript: hrefs (real browser strips both). A reader will assume inline handlers are the only environment gap. Additionally the test proves the sanitizer is in the path via an environment-dependent payload; asserting markdown was processed (e.g. <strong> present) would prove the same wiring independent of DOM implementation.'
severity: minor
resolution: 'Extended the test comment to name the javascript: href gap alongside the onerror one, and added an environment-independent wiring assertion (expect ''<strong>bold</strong>'') so the test proves renderMarkdown is in the path without depending on happy-dom''s sanitization behavior. Discovered while doing so that happy-dom ALSO keeps a <script> when it follows inline content (''**bold** <script>'' survives; a bare ''<script>'' does not) — verified under jsdom that both are stripped correctly there, so it is the same environment defect. The two payloads are therefore kept on separate fields (header proves markdown processing, footer proves script stripping) and the comment documents why.'
status: addressed
---
