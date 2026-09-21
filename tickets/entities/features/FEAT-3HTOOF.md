---
id: FEAT-3HTOOF
type: feature
title: CLI-first agent workflow for rela's own development
summary: Agents working on rela drive its tickets and docs projects through the rela CLI rather than an MCP server. The CLI is dogfooded on every task and no MCP server needs to run.
description: CLAUDE.md directs agents to perform all ticket and doc CRUD through the rela CLI (rela --project=tickets ... / rela --project=docs-project ...) and the repo registers no rela MCP server. MCP tool definitions load into every session whether or not it touches tickets; CLI invocations cost context only when made. Routing real work through the CLI also surfaces defects an MCP-only workflow would never reach.
status: implemented
---

Agent instructions in CLAUDE.md direct all ticket and doc CRUD through `rela
--project=tickets ...` and `rela --project=docs-project ...`.

Two reasons to prefer the CLI over the MCP server here:

- MCP tool definitions occupy context in every session, whether or not the session touches tickets. The CLI costs nothing until it is called.
- Using the CLI for real work exposes its gaps. Defects that an MCP-only workflow would never surface become visible on ordinary tasks.
