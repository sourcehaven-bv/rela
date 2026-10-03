---
id: RR-0H1GNT
type: review-response
title: Duplicate scenario names pair up wrongly in diff
finding: Named scenario keys were not made unique, so two scenarios with one name compared as changed plus added.
severity: minor
resolution: keys adds an occurrence number whenever a key repeats; TestCompareDuplicateNames.
status: addressed
---
