---
id: RR-FD39ON
type: review-response
title: Guard advice recommends GetEntityAt which reads the zero face for a bare id
finding: The failure message named store.GetEntityAt as an alternative; for a bare id it calls GetEntityState(ctx, addr, ""). internal/cli/show.go does this and is not on the list.
severity: significant
resolution: The advice now says GetEntityAt with an ID@face address, and the checker doc lists GetEntityAt with a bare id as a known false negative.
status: addressed
---
