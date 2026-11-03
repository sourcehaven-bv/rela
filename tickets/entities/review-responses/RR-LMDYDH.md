---
id: RR-LMDYDH
type: review-response
title: PlainYAML11Bool refuses an explicitly tagged !!str yes
finding: 'internal/yamlscalar/yamlscalar.go:22 checks only the style, so confirm: !!str yes is refused, although the operator stated it is a string. confirmOf (buttonoptions.go:102) also falls through to it for the !!str tag. Fix: return false when n.Tag was set explicitly (n.Style&yaml.TaggedStyle != 0), and add a table case. Otherwise, document in the godoc that an explicit tag does not count as quoting.'
severity: nit
resolution: PlainYAML11Bool returns false for an explicit !!str tag (TaggedStyle set and ShortTag() == "!!str"); other explicit tags are still refused. Godoc updated. Table cases for !!str yes, the long tag form and !custom yes; realtimecfg 'tagged str yes is text' and dataentryconfig dashboard !!str cases; guides say quote or tag !!str.
status: addressed
---
