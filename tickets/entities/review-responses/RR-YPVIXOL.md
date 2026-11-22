---
id: RR-YPVIXOL
type: review-response
title: Piles group vanished when every pile is placed
finding: 'The group was left out when no unplaced pile remained. A user whose piles are all placed by pile: entries lost the + to make another.'
severity: significant
resolution: Sidebar.vue now shows the group once the user has any pile; Sidebar.piles.test.ts covers a user whose only pile is placed.
status: addressed
---
