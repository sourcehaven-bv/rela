---
id: RR-SKLS4D
type: review-response
title: warnLoad logs an empty face in world and family modes
finding: The world mode could log the world that decided the row.
severity: nit
reason: A store.WorldScope carries no name at this layer; mode, type and id locate the fault, and the dataentry request log already names the world. Adding a name would widen the resolver API for a log field.
status: wont-fix
---
