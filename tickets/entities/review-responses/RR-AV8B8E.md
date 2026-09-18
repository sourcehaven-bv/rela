---
id: RR-AV8B8E
type: review-response
title: Icon colour test asserted a weaker property than its name
finding: Both assertions were gated on the attribute being present, so a glyph omitting fill and stroke
  entirely passed trivially — and omission is the likelier shape for hand-written geometry than a hardcoded
  hex. The test asserted 'no part contradicts currentColor', not 'draws in currentColor only'.
severity: minor
resolution: 'Rewrote it to assert the full property: a part is either FILLED (carrying both fill: currentColor
  and stroke: none) or INHERITING (carrying neither). Added a second test that no part pins a literal
  colour under any colour-valued attribute name, so a future glyph cannot introduce one under a name this
  file does not enumerate.'
status: addressed
---
