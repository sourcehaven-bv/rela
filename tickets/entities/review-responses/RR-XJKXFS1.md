---
id: RR-XJKXFS1
type: review-response
title: Wrong parent indent for a sequence item key
finding: 'For ''- key: >'' the parent indent was the dash column / so the block swallowed sibling keys and could disable the fix for the whole file.'
severity: significant
resolution: 'Blocks are now found by walking the parsed yaml.Node for FoldedStyle scalars. The block ends at the first non-blank line indented less than its content. Test case: sequence item key with siblings.'
status: addressed
---
