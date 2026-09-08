---
id: FEAT-R012DX
type: feature
title: Data-entry read paths are query-efficient on PostgreSQL
summary: Every SPA-triggered request against the postgres backend issues a bounded, observable number of SQL statements, projects only the columns it renders, and hits an index for its predicates.
description: 'The data-entry SPA drives the postgres backend through generic store methods that were designed for the filesystem store: full-row loads (markdown body included) for listings that render titles, per-row follow-up queries for relations and neighbours, and Go-side sorting/filtering after fetching everything. This feature covers (1) making the per-request query count observable so regressions are visible, (2) measuring the actual SQL against a realistic seeded project with ACL and worlds/faces, and (3) fixing the hot paths: header-only projections for listings, batched relation/neighbour loads, predicate pushdown and derived indexes where a query shape is static.'
priority: high
status: proposed
---
