---
id: DOCS-CDV005
type: docs-checklist
title: 'Docs: CalDAV deployment behind Pratique'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] ~~Godoc~~ (N/A: documentation-only ticket, no code)

## Project Documentation

- [x] `docs/caldav.md` — the full deployment guide: topology (rela owns no CalDAV-specific auth or transport), collection config, Pratique configuration, running rela behind it, issuing a credential, adding the account on macOS, and a "when it fails" section
- [x] Documented that TLS is mandatory for Apple Reminders (plaintext produces an endless 401 loop) and that the account needs an explicit `https://`
- [x] `docs/caldav-clients.md` — which clients speak VTODO at all, and their rich-text and refused-write behaviour

## External Documentation

- [x] ~~README~~ (N/A: linked from the CalDAV feature entry rather than duplicated)

**Docs verified:** the guide was followed end-to-end against a live Pratique
instance with both Apple Reminders and Thunderbird, and corrected where reality
diverged (the `--allowed-origin` requirement for proxied deployments, and the
RFC 6764 `/.well-known/caldav` passthrough).
