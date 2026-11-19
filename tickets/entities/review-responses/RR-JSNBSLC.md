---
id: RR-JSNBSLC
type: review-response
title: A link nested in a clickable card or row fires both actions
finding: The url widget renders inside a kanban card (itself a router link), the calendar entry button and linked-entity cells. A click on the link also opened the card side panel or navigated the row.
severity: significant
resolution: The link stops click propagation (@click.stop) and KanbanView.onCardClick returns early for a click inside a nested anchor. The e2e spec clicks the card link and checks the board URL is unchanged.
status: addressed
---
