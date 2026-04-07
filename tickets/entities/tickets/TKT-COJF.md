---
id: TKT-COJF
type: ticket
title: Harden rela-server against browser-based local attacks
kind: enhancement
priority: high
effort: l
status: done
---

## Summary

Security review of `rela-server` (the data-entry HTTP app) revealed several
issues that make it unsafe to run permanently on a local port while the user is
browsing the web. A malicious website can currently:

- Read live project events via the SSE endpoint (CORS reflects any Origin).
- Create / update / delete entities via cross-origin `fetch` (no CSRF protection).
- Open arbitrary local files via `/api/open-file?path=...` (no path containment).
- Reach the server from any network interface (binds to `0.0.0.0`, not loopback).
- Pivot via DNS rebinding (no `Host` header validation).
- Trigger arbitrary configured shell commands via `/api/command/{id}` (RCE-by-design,
unprotected by CSRF).

This ticket tracks the plan to mitigate all of the above.

## Findings

### Critical

1. **Server binds to all interfaces** — `cmd/rela-server/main.go:67` uses
`Addr: ":" + *port`, exposing the server on `0.0.0.0` despite the log message
saying `http://localhost:...`. Should bind to `127.0.0.1` by default.

2. **No CSRF protection on state-changing endpoints** —
`internal/dataentry/handlers_api.go` accepts JSON POST/PUT/DELETE with no
`Origin`/`Referer` validation, no CSRF token, no SameSite cookie. Any tab can
`fetch('http://localhost:8080/api/entities', {method:'POST', body: ...})` and
silently mutate the project.

3. **CORS misconfiguration on SSE** — `internal/dataentry/watcher.go:219-224`
reflects the request `Origin` and sets `Access-Control-Allow-Credentials: true`
on `/api/events`, allowing any website to subscribe to file/entity change events
(information disclosure).

4. **Path traversal in `/api/open-file`** — `internal/dataentry/commands.go:324-334`
accepts a `path` query parameter and opens it with the OS default app, with no
check that the resolved path is inside the project root. Combined with the CSRF
gap, a malicious site can
`fetch('http://localhost:8080/api/open-file?path=/Users/<u>/.ssh/id_rsa&action=reveal')`.

### High

5. **No Host header validation (DNS rebinding)** — None of the handlers check
`r.Host`. Even with loopback binding, an attacker can rebind `attacker.com` to
`127.0.0.1` and issue requests with `Host: attacker.com`, bypassing browser
same-origin assumptions.

6. **Command script execution unprotected by CSRF** —
`internal/dataentry/commands.go:260-271` runs `sh -c <script>` from configured
commands. Without CSRF, any malicious tab can invoke these as the user.

### Medium

7. **`WriteCacheFile` accepts unsanitised filename** —
`internal/repository/repository.go:368` joins the caller-supplied filename via
`filepath.Join(CacheDir, filename)`. Currently safe (only hardcoded callers),
but a footgun for the next handler.

8. **`RelationFilePath` uses relation type without explicit sanitisation** —
`internal/project/context.go` joins `from + "--" + relType + "--" + to + ".md"`.
`relType` is constrained by the metamodel today, but path construction itself is
not defensive.

9. **Server timeouts incomplete** — only `ReadHeaderTimeout` is set in
`cmd/rela-server/main.go`. Add `ReadTimeout`, `WriteTimeout`, `IdleTimeout` to
bound resource use.

## Already verified safe

- Markdown rendering: goldmark with default-safe HTML escaping.
- YAML parsing: `gopkg.in/yaml.v3`, no gadget chains.
- Entity ID validation: rejects `..` and path separators at creation time.

## Acceptance criteria

- [ ] Server binds to `127.0.0.1` by default; remote bind requires explicit `--bind`.
- [ ] All state-changing endpoints reject requests whose `Origin` (or `Referer`
fallback) is not in the allowlist.
- [ ] All requests with a `Host` header outside the loopback allowlist are rejected.
- [ ] `/api/events` no longer emits `Access-Control-Allow-Origin` or
`Access-Control-Allow-Credentials`.
- [ ] `/api/open-file` confines paths to the project root and rejects traversal.
- [ ] `WriteCacheFile` rejects filenames containing `/`, `\`, or `..`.
- [ ] All four `http.Server` timeouts are set in `cmd/rela-server/main.go`.
- [ ] Integration tests cover each rejection path (cross-origin, host spoof,
traversal).
- [ ] Documentation explains the threat model and `--bind` opt-in.

## Out of scope

- Authentication / multi-user access control.
- Per-instance session token (defence in depth, deferred to a follow-up).
- Wails desktop app (different IPC boundary; review separately if needed).
- Lua sandbox (covered by existing tests, see `lua-sandbox-tests`).
