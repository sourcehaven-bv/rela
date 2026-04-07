---
id: FEAT-ESLP
type: feature
title: Harden data entry server against local-network and browser attacks
summary: Make rela-server safe to run permanently on a local port by mitigating CSRF, DNS rebinding, CORS leakage, path traversal, and unauthenticated RCE via command scripts.
description: Make rela-server safe to run permanently on a local port. Currently a malicious website visited by the user can read live project events, mutate entities via cross-origin fetch, open arbitrary local files, and trigger configured shell commands. This feature delivers loopback-only binding, Origin/Host validation, CORS lockdown on SSE, path containment for file endpoints, and complete server timeouts.
priority: high
status: proposed
---

## Goal

When a user runs `rela-server` permanently on `localhost:8080`, a malicious
website they visit must not be able to read project data, modify entities,
execute commands, or open arbitrary local files.

## Threat model

- Attacker: arbitrary website loaded in the user's browser, executing JavaScript.
- Capability: cross-origin `fetch`/`EventSource`/form submissions, DNS rebinding.
- Out of scope: local malware running with user privileges; physical access.

## Requirements

1. Server binds to loopback by default; remote bind requires explicit opt-in.
2. State-changing requests (POST/PUT/DELETE) reject cross-origin callers.
3. SSE / streaming endpoints do not leak project events to other origins.
4. Host-header-spoofing / DNS rebinding attacks are rejected.
5. File-system endpoints (open-file, import) cannot escape the project root.
6. Command execution endpoint cannot be invoked by a third-party origin.
