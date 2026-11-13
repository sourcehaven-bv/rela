---
id: FEAT-QLU1C1B
type: feature
title: Declarative LLM actions
summary: 'Operator-configured LLM tasks: static prompt, gated JSON inputs, schema-constrained outputs'
description: 'Operators configure an llm: block on a data-entry action: inputs (properties, related entities, content), a static prompt, and outputs (set properties, create relations to chosen candidates). The model proposes; the user applies what they accept under their own ACL.'
priority: medium
status: proposed
---

## Motivation

Operators want LLM help on entities (triage, classify, suggest links) without
writing Lua. Today that needs a script action with `capabilities: ai`.

## Shape

A declarative `llm_tasks:` registry. Each task names a subject type, an
untemplated prompt, the inputs rela serializes as JSON (gated for the invoking
principal), and outputs constrained by a JSON schema derived from the metamodel:
set properties, relate to candidates chosen from a gated query. Actions
reference a task with `llm: <task>`.

## Principles

- The prompt is static text. Entity data travels as a separate JSON
message, never interpolated. No template language; logic belongs in Lua.
- Inputs are read through the visibility reader as the invoking principal,
so a hidden value cannot be laundered into a visible output.
- Outputs are limited to declared properties and to candidate ids; the
write goes through PatchEntity and normal validation.
- Human in the loop by default, per FEAT-ER3Y.
