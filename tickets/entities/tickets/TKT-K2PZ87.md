---
id: TKT-K2PZ87
type: ticket
title: Go 1.26.9 and x/net v0.60.0 for govulncheck findings
kind: chore
priority: high
effort: xs
started: "2026-10-09"
completed: "2026-10-09"
status: done
description: The daily govulncheck scan (#1817) found reachable vulnerabilities in net/http, crypto/tls, html/template, net/textproto and os (fixed in go1.26.9) and golang.org/x/net (fixed in v0.60.0). The auto-update workflow only runs go get, so it cannot bump the toolchain.
---

## Description

GitHub #1817. The daily govulncheck scan found reachable vulnerabilities
(GO-2026-6599 to GO-2026-6613) in the standard library, fixed in go1.26.9, and
in `golang.org/x/net`, fixed in v0.60.0.

The auto-update workflow runs `go get`, which cannot bump the toolchain, so it
filed the issue instead of a PR.

Change: `toolchain go1.26.9` in go.mod, every pinned `go-version` in
`.github/workflows`, and `golang.org/x/net` v0.60.0.
