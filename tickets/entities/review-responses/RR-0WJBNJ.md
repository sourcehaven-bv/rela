---
id: RR-0WJBNJ
type: review-response
title: A compose button into a cancelled or failed state loses its danger tone
finding: '`applyOptions(view, button, false, {})` drops the derived danger (AC 9). That matches the type button''s form path, but a compose "Abandon with reason" into a cancelled final state now looks like a neutral button. Its one-click sibling would be red. The danger tone is about the outcome, not about asking first, so consider passing `{danger: endTone(...).danger}` as derived with oneClick false. Or record why not in the plan.'
severity: nit
reason: 'A compose button follows the type button''s form path, where applyOptions with oneClick false drops the derived danger by design (TKT-TXZZZ2). Changing it only for compose buttons would make two form buttons into the same state look different. An operator who wants red sets style: danger, which a compose button keeps (AC 9).'
status: wont-fix
---
