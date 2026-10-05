---
id: RR-X9ESXX
type: review-response
title: Fused ID@face endpoints reach the manager and fail late
finding: lookupFamily strips a face from a fused id, so a relation write with from or to spelled ID@face was authorized and then rejected by the store as a malformed id. MCP create_relation passed to through raw.
severity: minor
resolution: 'The surfaces this PR touches refuse a fused endpoint before authorization: MCP create_relation refuses a faced to, and the Lua bindings refuse a fused from or to. The manager itself keeps accepting the fused form in lookupFamily because apply.go and the cascade host rely on it; the relation API flip (TKT-KQXVF7 PR 7) replaces these strings with RelationKey.'
status: addressed
---
