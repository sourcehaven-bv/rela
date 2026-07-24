---
id: RR-0GNDA9
type: review-response
title: Mermaid diagram-source injection via raw enum values/labels
finding: mermaidStateDiagram spliced raw enum values (from/to) and move labels directly into stateDiagram-v2 syntax. The whole diagram was HTMLEscapeString'd before <pre>, but renderMermaidDiagrams reads pre.textContent, which un-escapes before mermaid parses — so the escape protects only the HTML transport. A move label with a newline injects a spurious edge; a value with a space/arrow breaks parsing. Not XSS (mermaid securityLevel:strict), but corrupts/misrepresents the lifecycle diagram.
severity: significant
resolution: 'mermaidStateDiagram now aliases every state to a synthetic id (`state "value" as sN`) so raw values never appear as bare tokens, and flattens move-label newlines (mermaidLabel). The diagram displays the real values but the edges reference safe ids. Verified live: renders identically, no s0/s1 shown to the user. Tests: TestMermaidStateDiagram (aliased output) + TestMermaidStateDiagram_InjectionSafe.'
status: addressed
---
