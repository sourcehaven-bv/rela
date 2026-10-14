---
id: TKT-343IKH
type: ticket
title: Escape feed text in entity bodies
kind: enhancement
priority: medium
effort: s
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

## Description

Feed subscriptions place `item.title`, `item.authors`, `item.categories`,
`item.id` and `feed.title` into entity bodies with newlines removed but no
markdown escaping. A feed title such as `![](https://tracker.example/p.gif)`
therefore renders as a remote image for every viewer of the entity (RR-Q4FRPD on
TKT-RUO1GY). A raw query string in `item.link` can carry the same markup.

## Approach

- `ingest.Source.IsBlock(ref) bool` becomes `Kind(ref) ValueKind` with three kinds:
  - `ValueRaw`: flattened, never escaped. Every webhook value, so webhooks keep their behavior.
  - `ValueText`: flattened, and backslash-escaped where it lands in a markdown body (`create_if_missing.content`, `then.append_section.content`). Properties stay unescaped.
  - `ValueMarkdown`: sanitized markdown (htmlmd output). It keeps its lines in `create_if_missing.content` and is never escaped.
- Subscription text fields are `ValueText`; summary and content are `ValueMarkdown`; links, timestamps, `now`, `today` and `subscription.name` are `ValueRaw`.
- Links percent-encode the markdown punctuation in the query and fragment, so a URL cannot carry link or image syntax into a body.

## Acceptance criteria

1. A feed title `![x](https://t/p.gif)` in create content renders as literal text, not an image.
2. The same title in a property is stored unchanged.
3. Webhook payload values in a body are unchanged (existing tests pass).
4. A link with `![x](...)` in its query reaches the body without unescaped `[` or `]`.
