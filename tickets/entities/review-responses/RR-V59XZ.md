---
id: RR-V59XZ
type: review-response
title: Default value-builder allows markdown injection from entity titles
finding: buildEntityRefValue at markdown.go:1769-1774 splices the entity title into a markdown link without escaping. An entity titled ']"](javascript:alert(1))[evil' produces []"](javascript:alert(1))[evil](#anchor) — a complete malicious link. Verified via reproducing test. Anyone with permission to set entity titles can inject arbitrary markdown links, including XSS payloads when the rendered HTML is shown in a browser. Titles with [, ], (, ) also produce visibly broken links even without malice.
severity: critical
resolution: Added escapeMarkdownLinkText helper that escapes \, [, and ]. buildEntityRefValue now passes the title through this helper before splicing into the link-text slot. Custom format callbacks remain responsible for their own escaping (documented). Added test TestMdEntityRefs_TitleInjection verifying that a title with malicious link syntax round-trips through parse+render without breaking out of the link.
status: addressed
---

# Finding

`buildEntityRefValue` (`internal/lua/markdown.go:1769-1774`) does:

```go
return "[" + title + "](#" + anchor + ")", nil
```

with no escaping. The title comes from entity properties — user data.

**Reproduction (verified):** Entity titled `]"](javascript:alert(1))[evil`
produces:

```
[]"](javascript:alert(1))[evil](#javascript-alert-1-evil)
```

That parses as a complete `javascript:` link followed by garbage. If the
rendered output is displayed as HTML in a browser (data-entry web app, exported
docs, MCP-driven viewers), this is XSS.

The docs say "the caller is responsible for any markdown escaping" — fair for
the `format` callback path, but the default `style="title-slug"` and
`style="id"` paths ARE the callers. They must escape themselves.

# Resolution

Escape the title before splicing into the link-text slot. At minimum:

- `\` → `\\`
- `[` → `\[`
- `]` → `\]`

(Markdown link-text doesn't need to escape `(` or `)`, but we should still
escape them defensively because some renderers are lax.)

Add a test where an entity title contains `]`, `[`, `(`, `)`, `\` and verify the
resulting markdown re-parses to a single link with the original title text.

Document the escaping behavior so users know what to expect for custom formats
(where they remain responsible).
