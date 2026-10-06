---
id: RR-EIYCYO
type: review-response
title: Command helpers reload the schema snapshot
finding: relationsForEntity and viewRelations called h.schema().Meta again instead of using the snapshot captured by the handler.
severity: minor
resolution: buildEntityInput and buildViewInput take the handler's s.Meta.
status: addressed
---
