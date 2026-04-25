---
id: RR-1ZKVX
type: review-response
title: Duplicated shortMetamodel YAML in test
finding: TestCreateEntity_CustomIDRejectedForShort duplicates a shortMetamodel YAML literal that appears in TestGenerateID_ShortWithIDCaps. Extract a shared helper.
severity: nit
reason: The YAML literal differs slightly (this one has just one entity type; the other has three). Extracting a helper for two call sites adds indirection without simplifying either. Can be revisited if a third call site appears.
status: wont-fix
---
