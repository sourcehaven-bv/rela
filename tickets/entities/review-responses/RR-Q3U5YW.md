---
id: RR-Q3U5YW
type: review-response
title: Lazy <img> with no dimensions shifts layout and dispatches no rendered event
finding: 'renderMermaidDiagrams dispatches rela:mermaid-rendered so scroll-to-anchor re-settles after layout shift. The plantuml path injects <img loading=lazy> with no width/height and dispatches nothing, so it shifts layout (asynchronously, on lazy load) with no signal — deep-link anchors land wrong. Fix: dispatch an analogous event on img load (and/or reserve space via CSS).'
severity: significant
resolution: img 'load' listener dispatches rela:plantuml-rendered (bubbling); router/index.ts scroll-settle now listens for it alongside rela:mermaid-rendered (added + removed in cleanup). Re-settles scroll-to-anchor after the lazy image shifts layout.
status: addressed
---
