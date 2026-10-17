---
id: RR-T8RZ8K
type: review-response
title: Boot type-record check split in two
finding: 'checkRecordedRefs duplicates checkChannelTypeRecords: two boot reads and two error formats for one rule.'
severity: nit
resolution: checkRecordedRefs is removed; checkChannelTypeRecords reads the store once and boot and reload share one helper and error format.
status: addressed
---
