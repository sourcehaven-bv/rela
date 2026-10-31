---
id: RR-MIV014E
type: review-response
title: Inline title save goes to the previous entity
finding: After a switch, a title committed on the new entity was PATCHed to the previous one. The title property found it about one run in five.
severity: critical
resolution: 'Root cause: pinEntityForFlush was a ref, so value === pin compared a reactive proxy and the pin was never released; every save after a switch went to the previous entity. It is a shallowRef now. EntityTitle is keyed on servedRef so an open edit cannot commit onto another entity, and landView flushes held edits to the old row before swapping. Reproduced on develop''s component. Superseded when develop was merged in: BUG-NSGD6OE (#2040) gives each served address its own autosave instance, so the pin and the flush before swap were removed; EntityTitle stays keyed on servedRef.'
status: addressed
---
