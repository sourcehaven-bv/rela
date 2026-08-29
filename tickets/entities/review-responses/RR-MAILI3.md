---
id: RR-MAILI3
type: review-response
title: Durable child payload serialized stale action authority
finding: Carrying action configuration and capabilities in a durable child lets a queued job execute instructions that an operator removed or narrowed after enqueue. It also enlarges the payload with authority-bearing data that the worker could accidentally trust.
severity: significant
resolution: Child payloads carry only task occurrence and selected-entity identifiers. The handler reloads the current declaration and re-derives capabilities principal and ACL request. Removed or incompatible declarations stop safely.
status: addressed
---
