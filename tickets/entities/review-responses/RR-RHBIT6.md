---
id: RR-RHBIT6
type: review-response
title: Pushed-vs-Go comparison ran on memstore only
finding: The realistic shape (gated EndpointMatch plus ACL HasInbound in one SQL statement) was never compared end to end on postgres.
severity: minor
resolution: Fixture takes a base store; TestListPushdown_ScopeMatchesGoPath_Postgres runs the same 288-case comparison over pgstore.
status: addressed
---
