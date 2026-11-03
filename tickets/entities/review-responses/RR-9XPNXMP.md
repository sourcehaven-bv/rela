---
id: RR-9XPNXMP
type: review-response
title: Grouping onto a card that only looks loose moves it out of its real group
finding: useWallGrouping decided 'loose' from columnOf, which is Other for a card whose group is not loaded (offered by another page, or just created). Dropping on it made a new group and re-pointed the card out of its real group.
severity: significant
resolution: 'groupNow reads the raw edge; a card with an edge to a group this board does not show is refused with ''kanban.groupElsewhere''. Test: ''refuses to group a card that is in a group this board does not show''.'
status: addressed
---
