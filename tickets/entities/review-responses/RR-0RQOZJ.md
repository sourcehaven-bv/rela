---
id: RR-0RQOZJ
type: review-response
title: Unknown PropOp defaults to equality
finding: Backends treat an unknown op as equality; queryplan/listpushdown lack explicit refusal (D7).
severity: minor
resolution: Each backend handles every operator explicitly; unknown operators return ErrInvalidQuery (sqlite falls back to naive). Planners refuse refs via StringShaped. Tests in storetest, naive, pg, sqlite and planners.
status: addressed
---
