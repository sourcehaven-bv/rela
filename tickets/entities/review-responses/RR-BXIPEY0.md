---
id: RR-BXIPEY0
type: review-response
title: Panic when a block is indented 82 columns or more
finding: breakPoint got a negative limit when the block indent exceeded foldWidth and indexed the string with a negative index.
severity: significant
resolution: 'wrapFolded leaves a line alone when the indent is half the width or more / breakPoint clamps its start to 1. Test case: block indented past the width.'
status: addressed
---
