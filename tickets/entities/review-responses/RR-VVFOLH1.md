---
id: RR-VVFOLH1
type: review-response
title: 'Code: one undecodable row stops the heal'
finding: A row whose scan or decode failed aborted the pass. It sorts first on every pass, so it blocked every row behind it.
severity: minor
resolution: Both backends log and skip such a row and still count it as read, so a batch of only such rows triggers the stall warning. TestElementHealSkipsUndecodableRow (sqlite) pins it.
status: addressed
---
