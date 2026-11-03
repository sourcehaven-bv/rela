---
id: RR-XPFA01
type: review-response
title: A type the page links over two relations is hidden from the Create menu
finding: 'On an entity page whose tabs link a type over two different relations, pageCreateLink resolves no link on a tab that has neither, so the menu now leaves the type out (e2e: Task on the feature page''s Blockers tab). Before this change the type was offered and created unlinked. A user on such a tab cannot create the type from the menu at all, and the docs did not say how to get it.'
severity: minor
reason: 'Hiding the type is the ticket''s first acceptance criterion: the menu offers only types a link resolves for on the open tab. Offering it unlinked would bring back the orphan creation this ticket removes, and picking one of the two relations would guess the author''s intent, which pageCreateLink deliberately refuses. The docs (GUIDE-data-entry § Entity pages) now name this case and the two ways to create the type: open a tab that has one of the relations, or create it outside the entity page.'
status: wont-fix
---
