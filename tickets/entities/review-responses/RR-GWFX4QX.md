---
id: RR-GWFX4QX
type: review-response
title: Boards ignore $user
finding: KanbanView applies static filters in the browser with a literal compare, so a board filter on $user showed no cards, while FixedFiltersEditor (used for lists and boards) says $user works.
severity: significant
resolution: KanbanView resolves $user through resolveUserToken and useMeFor; with no identified user it matches no card. Tests in KanbanView.test.ts (own cards, unknown, failure, no /_me request without the token).
status: addressed
---
