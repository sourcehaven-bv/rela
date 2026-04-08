---
id: ai-integration
type: concept
title: AI Integration
summary: Integration with LLM providers via OpenAI-compatible APIs, exposed to scripts and tooling for content generation, analysis, and transformation.
description: Unified surface for invoking LLMs from rela. Provides configuration (.rela/ai.yaml) targeting any OpenAI-compatible provider (OpenAI, Anthropic, Ollama, LM Studio, Groq, etc.) and exposes LLM access to scripts (Lua), CLI commands, MCP tools, and the data entry UI in successive slices. Designed to be local-first, human-in-the-loop, auditable, and cost-aware.
layer: core
status: draft
---

## Purpose

A unified surface for invoking large language models from within rela. Enables
scripts, validations, and (eventually) CLI/UI features to call LLMs for tasks
like:

- Drafting and improving entity descriptions
- Suggesting missing relations
- Detecting duplicates and inconsistencies
- Generating documentation from structured entities
- Analyzing content for quality, completeness, and risk

## Provider Strategy

rela targets the **OpenAI Chat Completions API** as the lingua franca. Most LLM
providers (Anthropic, Groq, Together, Mistral, local runtimes like Ollama and LM
Studio, etc.) either expose this API natively or have well-maintained
compatibility layers. Targeting one wire format keeps the integration small
while supporting the entire ecosystem.

## Configuration

Provider configuration lives in `.rela/ai.yaml` (gitignored, per-user) so
credentials never enter the project repository. The config specifies the base
URL, default model, and (optionally) the environment variable holding the API
key. When `api_key_env` is omitted, no `Authorization` header is sent — this
supports local providers like Ollama and LM Studio that run without
authentication.

## Security: Network Egress as a New Threat Class

Adding AI to the Lua sandbox is **not** a no-op against the existing threat
model. Before AI integration, a malicious Lua script could only damage the
local project (read entities, write files within project root). After AI
integration, a malicious script can additionally call `ai.chat` to silently
exfiltrate every entity in the project to **the user's own legitimate
provider**. The data lands in the provider's logs — possibly in training data,
possibly readable by junior staff, possibly billed to the user. The script
needs no malicious config, no filesystem write, no separate compromise. It
uses the user's own working setup.

This is a meaningful escalation, not a no-op. Mitigations in the foundation
slice (TKT-YBKB):

- Operational logging makes unusual call patterns visible (DEBUG/INFO/WARN with structured fields)
- API is opt-in: requires `.rela/ai.yaml` to exist
- API key never logged or echoed in errors (`redactKey` helper + table-driven leak test)
- Config rejects URLs with embedded credentials (`https://user:pass@host`)
- Response body capped at 10 MiB

Mitigations deferred to follow-up tickets:

- One-time warning on first AI invocation per script or session (needs UX design)
- Allowlist of script files permitted to use `ai.*`
- Per-script token / call budgets
- Network egress audit log

**Treat Lua scripts as trusted code.** The combination of `rela.write_file`
and `ai.chat` means a malicious script can exfiltrate or corrupt project data.

## Surface Areas

The integration is built up in slices:

1. **Lua bindings** (first slice): `ai.chat()` and `ai.complete()` available inside Lua scripts. Enables user-written automations, validations, and views to call LLMs.
2. **CLI commands** (future): `rela ai ...` for ad-hoc tasks like description improvement and link suggestion.
3. **MCP tools** (future): expose AI helpers to AI assistants like Claude Code.
4. **Data entry UI** (future): inline "✨ Suggest" buttons for property completion and relation discovery.

## Design Principles

- **Local-first option**: Ollama and LM Studio support out of the box — no data leaves the machine unless the user opts into a hosted provider.
- **Human-in-the-loop**: AI output is always a *proposal*, never a silent edit. Destructive actions remain explicit.
- **Auditability**: AI calls should be loggable so users can trace why a suggestion was made.
- **Cost-aware**: Cache where possible; prefer cheaper models by default.
