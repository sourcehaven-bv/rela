---
id: TKT-TLQ94B
type: ticket
title: 'Top-of-stack smoke tests: MCP dispatch, router walk, ServeHTTP test convention'
kind: test
priority: medium
effort: s
status: done
---

## Problem

A test-quality review found that the interface-layer tests sit one layer below
the surface they claim to test:

1. **MCP** (highest value — zero coverage at any level): tests call handler methods directly (`s.handleListEntities(ctx, …)`) with hand-built `map[string]interface{}{"limit": float64(2)}` arguments that *simulate* JSON decoding. The real tool registration → dispatch → argument-decode path of the mcp-go server is never exercised; a tool registered under the wrong name, or with a schema/handler mismatch, is invisible to the suite. No e2e covers MCP.
2. **dataentry router**: ~127 tests in `api_v1_test.go` call `app.handleV1*` methods directly with pre-parsed route params, bypassing mux registration, URL-pattern parsing, method routing, and middleware. Playwright e2e covers SPA-used routes (slowly, with obscure failure modes), but the v1 API is a public API — routes the SPA doesn't exercise have no router-level coverage. Additionally, the `TestAppRouter_*` test family misleadingly suggests router coverage while calling handlers directly.
3. **CLI**: covered end-to-end by the Demos CI job (`scripts/demo-*.sh`, `scripts/e2e-*.sh`) — real argv → kong parsing. No new mechanism needed; residual gap is commands no script touches (see audit note below).

## Approach (agreed with reviewer in session)

Additive, no rewrite of existing tests:

1. **MCP dispatch test**: build the real server via the production registration call, invoke every registered tool through the server's actual dispatch entry (real JSON arguments, one happy-path call per tool, table-driven). Fallback if no ergonomic dispatch entry: feed JSON-RPC `tools/call` messages through the message handler.
2. **Router walk test**: table-driven through `app.NewRouter().ServeHTTP` for every registered API route — assert not-404/405, well-formed response envelope, no-cache middleware header. Table fails loudly when a new route isn't added.
3. **Rename `TestAppRouter_*`** to reflect what they test (handler-level affordance behavior).
4. **`doRequest` helper** going through `NewRouter().ServeHTTP` + convention note: new endpoint tests use it; old tests migrate opportunistically.

Explicitly out of scope: rewriting existing handler-level tests (their
assertions are altitude-appropriate; the wiring gap is O(routes) and closed once
by the walk test), CLI argv smoke (covered by Demos job).

## Verification

- New tests fail when: a route is removed from the mux, a tool is unregistered or renamed, an argument schema stops decoding.
- `just ci` green; CI green on PR.
