---
id: BUG-AH67
type: bug
title: Harden Lua sandbox security
description: |
    The Lua runtime sandbox needs hardening to prevent potential security issues.
priority: high
why1: |
    The initial Lua implementation loaded all standard libraries including io, os, debug
why2: |
    Scripts could use raw* functions to bypass protections on the rela module
why3: |
    File writes were allowed anywhere in the project root
why4: The initial Lua integration was scoped as a feature spike rather than a sandbox-first design
why5: Embedding a scripting runtime requires an explicit threat model and sandbox spec before exposing it to users
prevention: Added sandbox tests, restricted file writes to output/, removed dangerous functions
status: done
---

Security hardening for the Lua scripting feature:

- Remove dangerous libraries (io, os, debug)
- Remove dangerous functions (load, loadfile, dofile, rawget, rawset, etc.)
- Restrict file writes to `output/` directory only
- Use os.Root for traversal-resistant script loading
