---
id: RR-F7YWOL
type: review-response
title: A board watches only its first 100 cards; moves on later cards still need a reload
finding: KanbanView subscribes with page=1 and per_page=100 while it loads every page (frontend/src/views/KanbanView.vue:236-254). The busy wake is built from the replay's rows, so a session move on card 101 or later selects nothing. The docs change in AC 11 says that an open board updates live on a move. Add this to the Edge cases section and to both docs paragraphs ("cards past the first 100 refresh on the next entity change or reload"), as the existing lease comment in KanbanView already does.
severity: minor
resolution: 'Adopted: the 100-card limit is in Understanding, Scope out, Edge cases, AC 11 and both docs paragraphs, saying cards past the first 100 refresh on the next entity change or reload.'
status: addressed
---
