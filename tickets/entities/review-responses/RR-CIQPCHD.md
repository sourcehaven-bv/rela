---
id: RR-CIQPCHD
type: review-response
title: Transition when per row on list
finding: Performable binds related() per row; primeVerdicts does not prime transition traversals.
severity: significant
resolution: Add a page-level transition traversal prime (one Bind for all page ids) carried on ctx; budget fixture uses a related() when.
status: addressed
---
