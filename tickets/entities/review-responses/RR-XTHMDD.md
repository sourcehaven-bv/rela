---
id: RR-XTHMDD
type: review-response
title: '{title} is the display title on the detail page but the raw id in the form'
finding: EntityDetail.vue textVars sets title from entityDisplayTitle; DynamicForm.vue notEditableNote sets title to bareEntityId. The same `messages.read_only` string renders differently on the two surfaces. WorldMessages godoc defines {title} as the display title, so the form is wrong.
severity: significant
resolution: DynamicForm records entityDisplayTitle(entity) when it loads the row and substitutes that for {title}; the bareEntityId computed had no other reader and is removed.
status: addressed
---
