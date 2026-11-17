---
id: RR-JRDC1AM
type: review-response
title: Wildcard read lists declared types only
finding: visibleListByTypes with the wildcard scope now reads svc.Meta.EntityTypes(), so stored rows of an undeclared type drop out.
severity: minor
resolution: Intended\; documented on visibleListByTypes.
status: addressed
---
