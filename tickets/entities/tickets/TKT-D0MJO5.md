---
id: TKT-D0MJO5
type: ticket
title: 'seqtrace: text call trees and flow diffs for agents'
kind: enhancement
priority: low
effort: m
started: "2026-10-03"
completed: "2026-10-03"
status: done
---

## Description

seqtrace diagrams are Mermaid, which suits people but costs an agent many tokens
and gives it nothing to compare. Add an output for agents:

- `seqtrace diagram` also writes `NNN-name.txt`: a compact, indented tree of
the cross-package calls per scenario, with argument and result summaries. It
applies the same collapse and fold rules as the diagram.
- `seqtrace diff BASE HEAD` compares two diagram directories scenario by
scenario and prints what changed, by call shape (values ignored by default,
since IDs differ between runs).
- `just seqtrace-compare REF` runs the demo for a git ref and for the working
tree and diffs them, so an agent or reviewer can check that a change moved
request flows as intended (for example, a new path past `visibility`).
