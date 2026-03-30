---
id: BUG-AH67
status: done
title: Harden Lua sandbox security
type: bug
priority: high
description: |
  The Lua runtime sandbox needs hardening to prevent potential security issues.
why1: |
  The initial Lua implementation loaded all standard libraries including io, os, debug
why2: |
  Scripts could use raw* functions to bypass protections on the rela module
why3: |
  File writes were allowed anywhere in the project root
prevention: |
  Added sandbox tests, restricted file writes to output/, removed dangerous functions
---

Security hardening for the Lua scripting feature:

- Remove dangerous libraries (io, os, debug)
- Remove dangerous functions (load, loadfile, dofile, rawget, rawset, etc.)
- Restrict file writes to `output/` directory only
- Use os.Root for traversal-resistant script loading
