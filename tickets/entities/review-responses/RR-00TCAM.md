---
id: RR-00TCAM
type: review-response
title: No direct unit tests for the template scanner (parse/render agreement)
finding: parseDisplayTemplate/renderDisplayTemplate/collapseWhitespace were only tested indirectly via Parse/DisplayTitle. The nastiest inputs (nested/unbalanced braces {a{b}, stray } like a}b / {a}}, and the render-side unclosed-{ graceful-degradation branch) were uncovered, so the 'parse and render agree' invariant wasn't pinned directly.
severity: minor
resolution: Added TestParseDisplayTemplate, TestRenderDisplayTemplate (incl. the unclosed-brace verbatim-emit path), TestCollapseWhitespace, and TestEntityDef_DisplayProperties in types_test.go — table-driven, covering adjacent placeholders, leading/trailing/nested braces, unclosed, empty placeholder, and value-braces-not-rescanned.
status: addressed
---
