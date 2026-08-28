---
id: DOCS-DS1CR6
type: docs-checklist
title: 'Docs: Mail extensibility — HTTP and script transports, mail.send'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] `HTTPSender` documents why exactly one provider is compiled in, and why
      the base URL is not configurable
- [x] `ScriptSender` documents the no-graph-access property, the trust model,
      and why a script rather than a mapping DSL
- [x] `SendScriptPrincipal` records the secrets-scope and audit-identity
      decision with its rejected alternative
- [x] `lua.MailSender` / `registerMailModule` document why the binding is
      registered unconditionally, unlike the `ai`/`http` capability gate
- [x] `crypto.base64_decode` documents why it alone returns an error pair
- [x] `http` `form` / `basic_auth` documented in the package doc as general
      HTTP primitives, not mail-specific ones

## Project Documentation

- [x] `GUIDE-mail.md` — transport table, transport-choice guidance, `http` and
      `script` transport sections, credential sourcing per transport, the
      secrets-scope decision in operator terms, and troubleshooting entries
- [x] `GUIDE-lua-scripting.md` — `form`/`basic_auth` with both form shapes, the
      `crypto` module including base64, and a `mail.send` section
- [x] Generated `docs/mail.md` and `docs/lua-scripting.md` regenerated and
      verified by `just docs-check`

## External Documentation

- [x] `examples/mail/README.md` — how to install and adapt a send script, what
      a script gets and does not get, and how to report failure
- [x] The example scripts are labelled **examples, not supported
      integrations**, in both the README and the guide — they target
      third-party APIs rela cannot version, and the tests pin rela's contract
      rather than the providers'
- [x] ~~README~~ (N/A: the outbound mail guide is the public reference)

**Docs verified:** `just docs`, `just docs-check` and `just lint-md` all pass.
