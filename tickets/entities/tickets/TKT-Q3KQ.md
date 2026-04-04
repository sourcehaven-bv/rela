---
id: TKT-Q3KQ
type: ticket
title: Add shebang support to Lua scripts
kind: enhancement
priority: medium
effort: s
status: done
---

Allow Lua scripts in the scripts/ directory to start with a shebang line (e.g.,
`#!/usr/bin/env rela-lua`) for direct execution from the command line. The Lua
parser should skip shebang lines when executing scripts.
