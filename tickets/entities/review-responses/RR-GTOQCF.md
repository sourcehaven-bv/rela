---
id: RR-GTOQCF
type: review-response
title: Docs restated the valid icon names as an unpinned fourth copy
finding: |-
    I pinned the Go and SPA allowlists to each other in both directions, then wrote the same 15 names into prose twice (once in the kanban section, once in navigation) with nothing checking them — and the docs source makes copies four and five.

    Add an icon and the docs are wrong. That matters more than usual here because docs are the only place an author DISCOVERS these names; the startup error message is the only other source.

    The rename in RR-OX9WFS proved the point immediately: two of the names in those prose lists went stale the moment I changed them.
severity: minor
resolution: |-
    Fixed in bdb197f1. Both prose lists are gone, replaced by a pointer to the startup error message — which already interpolates sortedMapKeys(ValidIconNames), so it cannot go stale. The docs now say so explicitly, framing the omission as deliberate rather than an oversight the next person should 'helpfully' fill in.

    This also removes the last place the rename would have left a wrong answer.
status: addressed
---
