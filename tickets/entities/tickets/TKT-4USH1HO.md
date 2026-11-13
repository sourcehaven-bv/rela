---
id: TKT-4USH1HO
type: ticket
title: 'Declarative LLM actions: llm: block on data-entry actions'
kind: enhancement
priority: medium
effort: l
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

## Description

A data-entry action can carry an `llm:` block next to `set:` and `script:`. The
operator declares the inputs, a static prompt and the outputs. rela calls the
configured AI provider and returns a proposal. The user reviews the proposal in
a dialog and applies the changes they accept.

An `llm:` block has:

- `prompt`: static operator text. Entity data is never interpolated into it.
- `inputs`: subject `properties`, optional `content` with a size cap, and
`related` entities per relation (`direction`, `fields`, `limit`). rela sends
them as one JSON user message.
- `outputs`: `set` (subject properties) and `relate` (relation, `candidates`
condition, `limit`, `max`). rela derives a strict JSON schema from the
metamodel: enums stay enums, candidates become an enum of ids.
- Optional `model` and `max_tokens`.

## Rules

- The action requires `permission:`, because every run sends data to a
provider and costs money.
- Inputs and candidates are read through the visibility reader as the caller,
in the caller's world. A redacted field is never sent.
- Only properties the caller may write and relations they may create enter the
schema. With none, the action answers 403 `nothing_to_propose`.
- The reply is validated in Go against the schema. Unknown keys, invalid enum
values and ids outside the candidate set are refused (502 `ai_reply_invalid`).
- The action writes nothing. The SPA dialog applies the accepted changes
through PATCH and relation create, so the caller's ACL, validation and audit
apply. It re-reads the entity first and skips a property that changed since the
proposal.
- The provider call has a 2-minute timeout. Provider error text is never
returned to the caller.
- Faced subject or target types, content-scoped relations and relations with
required properties are refused at config load.
- `run_action` over MCP returns the same proposal.

## Out of scope (follow-ups)

- Auto-apply through `jobs.Queue`.
- `create` outputs, flow step outputs, list and batch runs.
- Server-level provider configuration.

## Implementation notes

- The proposal has one type, `actionproposal.Proposal`, shared by the
  data-entry API, MCP and the script layer. Its values are JSON
  (`json.RawMessage`), so no exported type carries untyped data.
