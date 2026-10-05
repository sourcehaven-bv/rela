---
id: RR-1IEC8C
type: review-response
title: 'YAML anchors, aliases and merge keys in faces: broke the load'
finding: 'declorder read the order from raw yaml.Node values without resolving aliases or << merge keys, so faces: *pf recorded no order and faces: {<<: *pf} recorded [<<, archived]. validateDeclOrder then refused schemas that loaded before (cranky review).'
severity: critical
resolution: mappingEntries follows alias nodes and expands merge keys in document position, with explicit keys winning. mappingKeys, mappingValue, recordFaceOrder and documentMapping use it. Pinned by TestDeclOrder_AnchorsAliasesAndMergeKeys, TestDeclOrder_AnchorsInIncludedFile and TestMappingEntries_ExplicitKeyWinsOverMerged.
status: addressed
---
