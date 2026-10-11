---
id: TKT-9DINM3
type: ticket
title: POST relations endpoint hard-rejects a type-allowlist mismatch
kind: enhancement
priority: low
effort: s
status: backlog
description: handleV1CreateRelation (write_handler.go) returns 422 on a target-type mismatch while PATCH writes the same edge with a warning (DEC-HWZHA). Pass RelationOptions.TolerateTypeMismatch and return the warning, or document why POST is strict. Found in review of BUG-X1479P (#1806).
---

## Description

`POST /api/v1/<type>/<id>/relations` (`handleV1CreateRelation`) hard-422s on a
type-allowlist mismatch. The PATCH relations path writes the same edge with a
`target_type_not_allowed` warning (DEC-HWZHA). Make the two agree, using
`entity.RelationOptions.TolerateTypeMismatch` from BUG-X1479P.
