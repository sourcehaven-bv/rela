---
id: DOCS-MAILD1
type: docs-checklist
title: 'Docs: Declarative scheduled mail'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Template model and recipient-scoped appbuild action documented
- [x] Raw address lookup versus ACL-visible content boundary documented

## Project Documentation

- [x] `GUIDE-mail.md` documents template schema, no-broadcast ACL model,
  recipient selection, validation, retry, and at-least-once delivery
- [x] Generated `docs/mail.md` updated and verified

## External Documentation

- [x] ~~README~~ (N/A: outbound mail guide is the public reference)

**Docs verified:** `just docs` and Markdown lint pass.
