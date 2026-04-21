---
id: RR-IXQ5Q
type: review-response
title: Lift reconcile to EntityManager so Lua/CLI/MCP can share
finding: The reconcile workflow is a concept EntityManager should expose; every caller (handlers, Lua scripts, MCP server) that wants 'save these relations declaratively' reinvents this loop today. Architect's C2.
severity: significant
reason: 'Deferred: architectural change on internal/entitymanager.EntityManager (add a Reconcile or Apply method) plus an audit of every caller (handlers, Lua bindings, MCP server, CLI). That''s a cross-cutting refactor on a public-ish interface and lands cleanest in its own ticket where the batch/transaction semantics (see also RR-KNXFF / architect C2) can be designed together. The current dataentry-local helper keeps the bug fix tight and does not lock in the shape of the eventual interface.'
status: deferred
---
