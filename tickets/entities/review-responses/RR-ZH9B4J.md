---
id: RR-ZH9B4J
type: review-response
title: A failed read sends the command an empty relation list
finding: relationsForEntity and viewRelations logged a relation read failure and returned no relations, and EndpointsReadable folded a header failure into all-false, so the script ran on stdin claiming the entity has no edges.
severity: significant
resolution: Both builders return the error and the handler answers 500 without running the command. The peer gate uses the new Resolver.EndpointsReadableErr, which returns a header or gate fault. Pinned by TestCommandInputs_ReadFaultFailsTheBuild and TestEndpointsReadableErr_ReturnsFaults.
status: addressed
---
