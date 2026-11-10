---
id: RR-H0L9XNQ
type: review-response
title: 'Code review: erase step for other hosts is a no-op'
finding: Running tenant erase on another host after the row is gone reports no tenant and removes nothing.
severity: minor
resolution: The guide tells the operator to remove the other hosts' audit directories by hand, using the schema name from erase or show.
status: addressed
---
