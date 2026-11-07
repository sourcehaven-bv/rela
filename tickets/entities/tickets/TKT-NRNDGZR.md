---
id: TKT-NRNDGZR
type: ticket
title: 'rela test v1 (2/4): injectable clock and determinism seams (mail transport, HTTP client, AI provider, output dir)'
kind: enhancement
priority: medium
effort: l
status: ready
---

## Description

Part 2 of TKT-0MCLX9S. The design is in PLAN-G8YMBDB.

Add appbuild options `WithClock`, `WithMailTransport`, `WithHTTPClient`,
`WithAIProvider` and `WithOutputDir`, usable with `NewInMemory`.

- **Clock:** reaches automation TemplateVars, every predicate evaluator (automation, validation, writerule, queryscopes, scopes), computed properties, Lua runtimes built for automations, actions and scripts, copy/convert interpolation, memstore timestamps and soft-delete, and the lease clock. Audit and version stamps stay on the wall clock.
- **Guard test:** an archguard test scans the write-path packages for `time.Now()`, with a reasoned allowlist.
- **Mail:** keeps parsing mail.yaml (RecipientPolicy applies) and replaces only the transport. Outbox delivery can flush synchronously.
- **HTTP:** a per-runtime `lua.WithHTTPClient` replaces the package-global client.
- **AI:** a caller-provided AI provider wins over ai.yaml.
- **Output:** `rela.write_file` honours the output-dir option.

## Acceptance criteria

- With a fixed clock, all of these see the same instant: `{{today}}` in an automation, `today()` in validation, writerule and scope predicates, a computed property, `rela.today()` in automation Lua, and memstore mod_time.
- Mail sent by an automation lands in the injected transport after a synchronous flush. A recipient blocked by RecipientPolicy stays blocked.
- A refusing HTTP client and AI provider are used by automation Lua that declares those capabilities.
- write_file with the option writes outside the project root.
- The guard test fails on a newly added time.Now in those packages.
