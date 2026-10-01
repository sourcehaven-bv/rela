---
id: RR-BP6P2K
type: review-response
title: yaml.v3 rewrite and parsing hazards
finding: Rewrite loses blank lines/indentation; yaml.Node accepts duplicate keys, anchors, aliases, merge keys and non-string keys.
severity: significant
resolution: AC3 narrowed (comments and order kept, blank lines not); parser rejects anchors/aliases/merge/duplicate/non-str keys with line numbers.
status: addressed
---
