---
id: RR-PT28IE
type: review-response
title: Two markdownToHTML pipelines (document.go, helpers.go) still diverge
finding: document.go markdownToHTML and helpers.go simpleMarkdownToHTML run different post-processing sequences; ConvertDiagramBlocks unifies only the diagram step, not the rest.
severity: minor
reason: 'Pre-existing tech debt unrelated to this ticket: helpers.go adds md-table class + checkbox indices, document.go doesn''t. ConvertDiagramBlocks fixes the diagram-block drift but the broader pipeline convergence is out of scope here. Follow-up ticket.'
status: deferred
---
