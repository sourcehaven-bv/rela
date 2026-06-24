---
id: RR-CIAI73
type: review-response
title: No <img> onerror handling — failure destroys the source block
finding: 'renderPlantUMLDiagrams replaces the <pre> with an <img> before any load succeeds. If the server is down/misconfigured/414s/returns non-image, the user sees a broken-image glyph and the source markdown is gone from the DOM. This is strictly worse than renderMermaidDiagrams, which console.errors and leaves the source block intact on failure. Fix: add img.onerror that restores a readable fallback (a <pre> with the source).'
severity: significant
resolution: 'Added img.onerror handler that replaces the wrapper with a <pre><code class=language-plantuml> carrying the original source, so server-down/414/non-image failures degrade to readable code instead of a broken-image glyph with the source lost. Test: ''restores a code block when the image fails to load''.'
status: addressed
---
