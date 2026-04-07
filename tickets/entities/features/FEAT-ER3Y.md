---
id: FEAT-ER3Y
type: feature
title: AI integration via OpenAI-compatible API
summary: LLM access throughout rela via configurable OpenAI-compatible providers, starting with Lua bindings.
description: 'Add a unified AI layer that lets rela call any OpenAI-compatible LLM provider. Configuration lives in .rela/ai.yaml. The integration is built up in slices: first as Lua bindings (ai.chat, ai.complete) so users can write scripts that invoke LLMs, then expanded to CLI commands, MCP tools, and UI features as concrete use cases emerge.'
priority: medium
status: proposed
---

## Motivation

LLMs are well-suited to tasks rela users perform constantly: drafting
descriptions, suggesting missing relations, detecting duplicates, generating
documentation, validating content quality. Rather than hardcoding any one of
these as a feature, expose LLM access as a primitive that users (and future rela
features) can compose.

## Strategy

**One wire format, many providers.** Target the OpenAI Chat Completions API.
This gives access to OpenAI, Anthropic (via compatibility layer), Groq,
Together, Mistral, Ollama, LM Studio, and most other providers without writing
per-provider code.

**Build up in slices.** Each slice ships independently and proves the design
before the next one is built.

## Slices

1. **Lua bindings** (this feature's first ticket)
   - `.rela/ai.yaml` config loader
   - `internal/ai` package with OpenAI-compatible chat client
   - `ai.chat({messages, model?, temperature?, max_tokens?})` Lua function
   - `ai.complete(string)` convenience wrapper
2. **Embeddings** — `ai.embed(...)` for semantic similarity (relation suggestion, duplicate detection)
3. **CLI commands** — `rela ai suggest-links`, `rela ai improve`, `rela ai ask`
4. **MCP tools** — wrap AI helpers as MCP tools so external agents can invoke them
5. **Caching layer** — content-hash-keyed cache to control cost
6. **Data entry UI integration** — inline AI affordances in the web UI
7. **AI-powered validation rules** — Lua validations that use `ai.chat` to flag quality issues

## Design Principles

- **Human-in-the-loop**: AI output is a proposal, never a silent edit
- **Local-first option**: Ollama / LM Studio work out of the box
- **Auditability**: Calls should be loggable for traceability
- **Cost-aware**: Cache aggressively, prefer cheaper models by default
- **Prompt-injection aware**: Entity content is user-controlled; never auto-execute destructive AI suggestions

## Open Questions

- Embedding model selection (text-embedding-3-small vs local alternatives)
- How to surface AI call cost / token usage to users
- Streaming API shape for long generations
- Whether to support tool use / function calling in the Lua binding
