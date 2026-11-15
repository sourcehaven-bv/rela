---
id: RR-FYQUDNX
type: review-response
title: DashboardView shows each card failed on a hidden anchor
finding: A hidden anchor makes every card answer 404 and DashboardView marks each card failed instead of one not-found state.
severity: minor
reason: The dashboard is only shown inside an entity page, whose header reads the anchor first and shows the not-found state. The per-card state is the existing behaviour for a card error.
status: wont-fix
---
