---
id: RR-U7358SX
type: review-response
title: Trigger decode errors report snippet line numbers
finding: 'parse.go decodeNode re-marshals a rule node and decodes the copy, so ''line N'' in strict-decode errors counts from the rule snippet, not from flows.yaml. Fix: decode the node directly so lines are the file''s.'
severity: minor
resolution: decodeNode decodes the yaml.Node itself and finds unknown keys with a tag-following walker, so reported lines are file lines. TestParse_DecodeErrorLines pins it.
status: addressed
---
