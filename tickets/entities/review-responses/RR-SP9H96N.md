---
id: RR-SP9H96N
type: review-response
title: Paging contract silent on ts changes
finding: The package doc promised stable paging without mentioning that a mutable row can move.
severity: minor
resolution: Package doc states that a row whose ts changes during paging may be returned twice or not at all.
status: addressed
---
