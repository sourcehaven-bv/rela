---
id: TKT-R8MEE2
type: ticket
title: 'Enable gosec G705 (XSS), add nosniff to feeds, render help via html/template'
kind: refactor
priority: medium
effort: s
status: done
---

## Description

`G705` was in the `gosec.excludes` block in `.golangci.yml`, so XSS taint
analysis never ran. Enabling it surfaced three findings, and two real hardening
changes came out of reviewing them.

**1. Missing `nosniff` on the feed responses.** `feed_handler.go` serves `.ics`
and `.json` feeds whose `Summary`/`Description` come straight off entity
properties (`feed_provider.go`), so anyone who can write an entity controls those
strings. The escaping itself is sound — iCalendar goes through
`calfeed.escapeText` (backslash-escapes `\ ; ,` and newlines) and `writeLine`
strips raw CR/LF so injected CRLF cannot forge a property line; JSON uses
`encoding/json`, which escapes `<`, `>`, `&` by default. But neither response set
`X-Content-Type-Options: nosniff`, and no middleware supplies one
(`middleware_security.go` sets none). Without it a browser may MIME-sniff a
`text/calendar` body containing `<script>` into `text/html`.

**2. The `handlers.go` `template.HTML` assertion did not hold as written.** The
upstream is `simpleMarkdownToHTML`, which runs goldmark with
**`html.WithUnsafe()`** — raw HTML passes through unsanitized, and there is no
HTML sanitizer anywhere in the repo. So this was *not* a verified-sanitizer case
and must not be annotated as one. What makes it non-exploitable is the input's
trust, verified rather than assumed: the value is
`metamodel.EntityDef.Description`, loaded only by `yaml.Unmarshal` from on-disk
`metamodel.yaml`. No HTTP/MCP/Lua path can write it — the only metamodel writers
are CLI-only, none imported by `dataentry`/`mcp`/`lua`/`rela-server`, and Lua's
`rela.write_file` is confined to `outputDir` by `filepath.IsLocal`.

## Solution

- Set `X-Content-Type-Options: nosniff` on both feed handlers.
- Because the safety argument rested on input trust rather than escaping, fix the
underlying smell instead of annotating it: render the help fragment through
`html/template` with contextual auto-escaping, replacing ~15 hand-escaped
`fmt.Fprintf` calls where every field was an individually-auditable decision. The
`template.HTML` seam is now explicit and documented as operator-trusted-input.
- Remove `G705` from the `gosec.excludes` block.
