---
id: TKT-OQ7MDF
type: ticket
title: MCP tool handlers return raw error text to the client
kind: enhancement
priority: medium
effort: m
tags: security
status: backlog
description: 'Sanitize MCP tool error replies in one pass: log the cause and return a classified message.'
---

## Description

About 46 MCP tool handlers in `internal/mcp` return `err.Error()` to the client.
The text can carry internal detail: store and gate faults, SQL errors, file
paths. Data-entry handlers log the cause and return a generic message instead.

Found in TKT-5LW875 review (RR-X873OB): `handleAnalyzeCardinality` returns raw
gate error text, and a gate fault now reaches that path. Fixing one tool would
leave the surface inconsistent, so this is one pass over the MCP error contract.

## Approach

- Define the MCP error contract: which errors are client-facing (validation,
not found, forbidden) and which are logged with a generic reply.
- Route every tool handler through one helper that applies it.
- A hidden entity stays indistinguishable from a missing one.

## Acceptance

- No MCP tool handler returns `err.Error()` of an unclassified error.
- A test injects a store fault into a representative set of tools and asserts
the reply carries no fault text and the log does.
