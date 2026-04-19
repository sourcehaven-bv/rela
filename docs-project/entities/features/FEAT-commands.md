---
id: FEAT-commands
type: feature
title: "Data Entry Commands"
status: published
summary: "User-defined scripts executed from the data entry UI with context-aware input and streamed results"
---

Configurable shell commands defined in `data-entry.yaml` under the `commands:` key.
Commands execute with entity, list, view, or global context, receive structured JSON
on stdin, and communicate results back via the `::rela::` line protocol on stdout.
Results stream into stacked toast notifications supporting messages, file open/reveal,
entity links, grouped output, and cancellation.
