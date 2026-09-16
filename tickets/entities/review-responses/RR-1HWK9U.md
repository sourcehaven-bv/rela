---
id: RR-1HWK9U
type: review-response
title: syncAttrs collapsed the three-valued checked attribute to two
finding: '`syncAttrs` wrote `String(node.attrs.checked === true)`, always emitting "true" or "false", while the preset''s `toDOM` writes the raw value and its `parseDOM` reads `dom.dataset.checked ? dom.dataset.checked === ''true'' : null`. Harmless today because the node view only runs for task items, but the comment claimed it reproduced the preset''s contract rather than replacing it, and it did not: a future change constructing a view for a non-task item would emit `data-checked="false"` and make a plain bullet parse back as an unchecked TASK item. The sibling lines had the same problem, writing `String(x ?? '''')` so an unset attribute became an empty string.'
severity: significant
resolution: All four now stringify the attribute as-is, so a copy of the DOM parses back to the same attributes. The comment states specifically why `checked` must not be collapsed, rather than making a general claim about reproducing the contract.
status: addressed
---
