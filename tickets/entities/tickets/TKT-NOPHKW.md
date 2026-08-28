---
id: TKT-NOPHKW
type: ticket
title: 'Enable gosec G704 (SSRF) and annotate operator-configured HTTP destinations'
kind: refactor
priority: medium
effort: s
status: done
---

## Description

`G704` was in the `gosec.excludes` block in `.golangci.yml`, so SSRF taint
analysis never ran. Enabling it surfaced two findings. Both are operator
configuration rather than SSRF, so no code behaviour changes — but each needed
its trust boundary traced end to end rather than assumed.

**`internal/ai/openai.go`** — the destination derives solely from
`Config.BaseURL`: `.rela/ai.yaml` (gitignored, per-user) → `ai.LoadConfig`
(plain `os.ReadFile` from local disk) → `ai.LoadProvider`. `LoadProvider` has
exactly one non-test call site (`internal/lua/context.go`), and the `.rela` dir
comes from `deps.ProjectRoot`, never from a request. Pointing rela at ollama /
LM Studio / a corporate gateway is the documented feature
(`docs/lua-scripting.md`), so a host allowlist would break the intended use case.

Lua cannot override the AI endpoint, confirmed negative at two independent
layers: `ai.ChatRequest` carries only `Messages`, `Model`, `Temperature`,
`MaxTokens` — no URL field on the request type, so no per-call override exists
even in principle — and the Lua binding `parseChatRequest` reads only
`messages`, `role`/`content`, `model`, `temperature`, `max_tokens` (same for the
embed path). So even fully untrusted Lua cannot steer the destination.

**`internal/cli/sync/client.go`** — base URL comes from the `rela sync
push/pull --remote` flag (env `RELA_REMOTE`), through `buildSyncEngine` →
`syncclient.NewClient`, which requires an absolute URL. The trust boundary is the
operator's shell — the same one that already grants full local filesystem access.

## Solution

- Annotate both sites with a narrow `//nolint:gosec` naming the specific trust
boundary, matching this repo's existing suppression style (as in
`internal/canonical/canonical.go`).
- Remove `G704` from the `gosec.excludes` block.
- No security control weakened: no allowlist removed, and the existing
`Config.Validate` constraints (http/https only, no credentials, query string or
fragment) and the client's refusal to follow redirects both remain.
