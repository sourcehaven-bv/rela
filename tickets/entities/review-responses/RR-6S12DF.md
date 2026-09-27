---
id: RR-6S12DF
type: review-response
title: Ordering index column order and DESC plans
finding: One column order must be shared by DDL and query; DESC with ASC id tiebreak cannot use a reverse scan.
severity: minor
resolution: One helper renders (txt IS NULL), rank/txt, id for both; EXPLAIN asserts no temp B-tree for ascending pages only.
status: addressed
---
