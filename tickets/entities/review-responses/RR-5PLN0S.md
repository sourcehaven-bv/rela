---
id: RR-5PLN0S
type: review-response
title: 'An empty interpolated block appended a stray blank line per delivery'
finding: 'If a template is content: "{{body.msg}}" and msg is absent, interpolate returns "" (deliberate — dropping the delivery would be worse) and AppendToSection inserted an empty element, so the section gained a stray blank line on every delivery. Over a few hundred alerts from a producer with an intermittently-missing field, the section accumulates whitespace in a document meant to be read by a human during an incident.'
severity: minor
resolution: 'Fixed as a side effect of the AppendToSection insertion change (RR-8KQ2MW): splitContentLines("") returns nil, so an empty block now inserts no elements at all and the document is returned byte-identical. Pinned by TestAppendToSection_EmptyBlockAddsNothing. Behaviour on a MISSING section is unchanged — the heading is still created — because that path goes through appendLines rather than the insertion.'
status: addressed
---

Fixed for free by making the insertion split the block: an empty block becomes
zero lines rather than one empty one.
