---
id: RR-MPYXUB3
type: review-response
title: 'Code: wrong statements in elementnaive doc'
finding: The package doc said one header-free endpoint read and that cost does not depend on rows in range.
severity: minor
resolution: 'Reworded: one full read of the endpoint rows; the number of store calls is constant; work and memory grow with the type; the calls are not one snapshot.'
status: addressed
---
