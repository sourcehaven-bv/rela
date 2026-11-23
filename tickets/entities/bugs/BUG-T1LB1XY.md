---
id: BUG-T1LB1XY
type: bug
title: Export items in the list more menu render in capitals
priority: low
effort: xs
why1: A scoped rule uppercased every export menu item.
why2: The rule was written for bare format names (pdf, docx) before the more mode used a sentence.
why3: The more mode reused the item class without checking the shared style.
prevention: A test asserts the uppercase class is only on bare format names.
status: done
---

Export items in the list more menu showed as EXPORTEREN ALS DOCX. Fixed in
ExportMenu.vue.
