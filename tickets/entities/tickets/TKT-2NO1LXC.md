---
id: TKT-2NO1LXC
type: ticket
title: 'rela test: stub and record AI calls'
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

Split out of TKT-HB313W5, which stubs HTTP. A case declares canned AI answers
with `t.ai.stub{...}` (match on the prompt, answer with text or an error kind)
and inspects the recorded calls, the same shape as `t.http.stub`. The test world
already injects an AI provider (`appbuild.WithAIProvider`) that wins over
`.rela/ai.yaml`; it refuses every call today.
