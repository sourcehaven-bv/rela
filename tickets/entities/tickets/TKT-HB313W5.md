---
id: TKT-HB313W5
type: ticket
title: 'rela test: stub and record outgoing HTTP and AI calls'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Tests declare canned responses with `t.http.stub{...}` and `t.ai.stub{...}`, and
assert on recorded requests. Needs an injectable `http.RoundTripper` in place of
the package-global `lua.httpClient`, and a test AI provider that takes
precedence over `.rela/ai.yaml`.
