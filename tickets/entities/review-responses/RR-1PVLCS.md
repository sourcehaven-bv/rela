---
id: RR-1PVLCS
type: review-response
title: readableRelations duplicates FilterRelations
finding: The command handler looped over EndpointsReadable itself instead of using the visibility wrapper.
severity: minor
resolution: The loop moved onto the visibility seam (visibleReader.readableRelations over Resolver.EndpointsReadableErr). FilterRelations is not reused because it folds a read fault into an empty answer, which the command path must not do.
status: addressed
---
