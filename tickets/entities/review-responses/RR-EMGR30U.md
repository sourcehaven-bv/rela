---
id: RR-EMGR30U
type: review-response
title: A filter during a move wiped hidden rows' places
finding: Placed kept only ids in the filtered rows so a move under a filter deleted every hidden row's position.
severity: significant
resolution: Stored ids absent from the rows keep their place after the id they followed. TestManualOrder_FilterKeepsHiddenPlaces pins it.
status: addressed
---
