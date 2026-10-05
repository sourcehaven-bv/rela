---
id: RR-XXXIV9
type: review-response
title: Family uses AllStates, which store.EntityQuery reserves for infrastructure
finding: The AllStates godoc says it is not for read paths choosing which face to show, and Family is such a read path.
severity: significant
resolution: 'The AllStates godoc now names visibility.Resolver.Family as the one read-path use and states why it is safe: headers only, and every unreadable face is dropped before anything leaves it.'
status: addressed
---
