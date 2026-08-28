---
id: TKT-N8RESF
type: ticket
title: CalDAV deployment documentation (rela behind Pratique)
kind: docs
priority: medium
effort: s
status: done
---

## Description

Document how to run rela's CalDAV endpoint in a real deployment. Design in
**RES-1Y2EB5** (Axis C).

**Docs only.** An earlier draft of this ticket also proposed an `--allowed-host`
flag and a startup refusal to serve CalDAV over plaintext. Both were dropped —
see "What this ticket deliberately does NOT do" below. The short version: a
CalDAV route under `/api/` is just another `/api/` route, and inventing
CalDAV-specific gating would contradict the reason it was mounted there.

## Context: rela owns no CalDAV-specific auth or transport

The chain is two components:

```text
CalDAV client --TLS--> Pratique --X-Auth-Assertion--> rela
```

- **Pratique authenticates.** Its Personal Access Token class was designed for
this by name (`docs/08-scoped-long-lived-tokens.md`): HTTP Basic with the
password as the token and the username display-only, because "CalDAV clients,
which must send a username, work" only under that rule.
- **Pratique terminates TLS.** As of its `feat/tls-termination` branch it does
this in-process — either `tls: {cert_file, key_file}` or `tls: {auto: ...}` for
ACME/Let's Encrypt with in-process renewal (the two are mutually exclusive; a
filled-in `auto` block without `enabled: true` is rejected rather than silently
ignored). **Verify this has landed before publishing the docs**; if it has not,
the deployment additionally needs a TLS terminator in front.
- **rela verifies the assertion.** `internal/jwtauth` checks the ES256 JWT
against Pratique's published JWKS with iss/aud guards; `requireVerifiedJWT` is
the fail-closed gate, and `-jwt-header` already defaults to `X-Auth-Assertion`.

So the CalDAV endpoint is a JWT-gated `/api/` route like any other. No new
credential store, no TLS code, no CalDAV-specific middleware.

## Deliverables

A deployment guide covering:

1. **The topology** — the two-component chain above, and why rela sees no Basic
credential at all (Pratique consumed it upstream).
2. **Pratique setup** — the PAT class enabled, a low-impact capability for
calendar/todo access (its shipped example config uses `calendar:read` /
`todo:read`), and TLS via cert files or ACME.
3. **Issuing a credential** — PATs are minted in Pratique's web UI under
account/tokens. There is deliberately no admin CLI for this: issuance requires
an authenticated session. The plaintext is shown **once**.
4. **The macOS client steps**, which are non-obvious and cost two failed
attempts to discover:
   - System Settings → Internet Accounts → Add Account → Other → CalDAV
   - Account Type: **Manual**
   - Server Address **must include the `https://` prefix**
   - Username is display-only (anything); the **password is the PAT**
   - Toggle **Reminders** on for the account
   - Note the failure mode: macOS reports *every* setup failure as "account
name or password verification failed" regardless of cause, so an operator
debugging this should read the server log, not the dialog.
5. **Constraints, stated plainly:**
   - **CalDAV requires a proxied deployment.** `rela-server --project .` alone
cannot serve Reminders: macOS will not send credentials over plaintext, and rela
has no credential subsystem by design.
   - **CalDAV is unavailable in `rela-desktop`.** It opens no network listener
at all — Wails drives the router in-process — so there is no socket for a client
to reach.
   - **The `Host` allowlist.** Behind a proxy the forwarded `Host` must be
accepted, which today means binding `0.0.0.0` (which sets `allowedHosts = nil`).
A narrower `--allowed-host` flag is tracked separately (see below) because it
affects every proxied deployment, not just CalDAV.

## What this ticket deliberately does NOT do

- **No startup refusal to serve CalDAV over plaintext.** By the time a request
reaches rela there is no Basic credential — Pratique consumed it and injected an
assertion. rela would be refusing a credential it never receives, based on a
transport property it cannot observe (TLS terminates upstream), on behalf of a
client it knows nothing about. The JWT gate already fails closed when the
assertion is missing or invalid, which is the actual protection. The TLS
requirement is real but it is a **client↔Pratique** property, so it belongs in
these docs, not in a rela startup check.
- **No CalDAV-specific gating of any kind.** Mounting under `/api/` was chosen
precisely so the endpoint inherits `attachACLRequest`, the read gate, the JWT
gate and principal stamping. Adding a bespoke check on top would contradict that
choice and give the endpoint a second, divergent security story.
- **No `--allowed-host` flag.** Split out: `requireLocalHost` has no path
exemption and no such flag today, so a proxied request carrying a forwarded
`Host` gets a 403 before any handler runs. That already affects the REST API and
the ICS feeds — CalDAV merely surfaces it. It is a general proxy-deployment gap
and deserves its own ticket described in those terms.
- **No fix for the hardcoded `http://` origin derivation**
(middleware_security.go:80-90). Latent while rela stays plaintext behind a
proxy; a trap for anyone later adding direct TLS to rela. Worth a comment at the
site, tracked with the `--allowed-host` work rather than here.

## Acceptance criteria

1. An operator can go from zero to a working Reminders account by following the
guide, including the `https://` prefix and PAT-as-password steps.
2. The docs state that CalDAV requires Pratique + TLS and is unavailable in
rela-desktop.
3. The docs do not imply rela performs any CalDAV-specific authentication or
transport check.
4. The Pratique TLS instructions match what has actually landed there.

## Out of scope

- Any credential storage or verification in rela.
- TLS termination in rela.
- The protocol surface (TKT-MF1CWZ).

## Outcome

`docs/caldav.md`, linked from the README docs table.

Acceptance criteria:

1. Zero-to-working-account walkthrough, including the `https://` prefix and
   PAT-as-password — yes (steps 1-5).
2. States that CalDAV requires Pratique + TLS and is unavailable in
   rela-desktop — yes (topology callout + Constraints).
3. Does not imply rela does CalDAV-specific auth/transport checks — yes; says
   the opposite explicitly, and explains why.
4. Pratique TLS instructions match what landed — yes. Verified against
   pratique `develop` (PR #4 "Terminate TLS in-process: cert files + optional
   ACME" is merged): `cert_file`/`key_file` and `auto.enabled` are mutually
   exclusive, `storage_dir` must persist. JWKS path corrected to
   `/.well-known/pratique/jwks.json` (web.go:639) — an earlier draft had it
   under the mount prefix.

All rela flags in the guide verified present in cmd/rela-server/main.go.
