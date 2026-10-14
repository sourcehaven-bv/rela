# Feed subscriptions

rela can poll RSS, Atom and JSON Feed sources and turn each item into an
entity. You declare the feeds in `subscriptions.yaml` in the project root. The
scheduler polls each one on its own interval.

A subscription uses the same mapping vocabulary as a
[declarative webhook route](webhooks.md): `find:`, `create_if_missing:` and
`then:`. The difference is the source. A webhook maps a request that arrives;
a subscription maps the items of a feed that rela fetches.

For a source that is not a syndication feed, write a
[scheduled task](scheduled-tasks.md) in Lua. `rela.http`, `rela.json`,
`rela.xml` and `rela.feed` cover the parsing.

## Example

```yaml
subscriptions:
  go-blog:
    description: The Go blog
    url: https://go.dev/blog/feed.atom
    every: 1h
    max_items: 20
    find:
      type: article
      match: [guid]
      values: {guid: "{{item.id}}"}
    create_if_missing:
      properties:
        title: "{{item.title}}"
        link: "{{item.link}}"
        published: "{{item.published_date}}"
      content: "{{item.summary}}"
```

The first poll creates one `article` per item. Later polls skip items they
have already imported, so a feed item becomes one entity.

## Keys

| Key | Meaning |
|-----|---------|
| `url` | The feed. `http` or `https` only, without credentials or a fragment. |
| `every` | The interval, in [schedules.yaml syntax](scheduled-tasks.md). The minimum is 15 minutes. |
| `max_items` | How many of the newest items one poll imports. Default 50, maximum 1000. |
| `auth` | Optional credential; see [Authentication](#authentication). |
| `description` | Free text for the operator. |
| `find`, `create_if_missing`, `then` | The mapping, as in [webhooks](webhooks.md#three-workflows). |

A subscription name uses lowercase letters, digits, `-` and `_`, and is at most
64 characters. The scheduler runs it as the task `subscription:<name>`, so no
task in `schedules.yaml` may use that name.

`find.match` is required. Without a match key, every poll would create a new
entity for every item.

## Template values

| Reference | Value |
|-----------|-------|
| `item.id` | The item's GUID or id; the link when the feed has none. |
| `item.title` | The title. |
| `item.link` | The link, made absolute against the feed URL. Only `http` and `https` links are kept. |
| `item.summary` | The summary, converted from HTML to markdown. |
| `item.content` | The full content, converted from HTML to markdown. Empty when the feed carries only a summary. |
| `item.published`, `item.updated` | RFC 3339 timestamps. |
| `item.published_date` | The publication date as `YYYY-MM-DD`. |
| `item.authors`, `item.categories` | Comma-separated lists. |
| `feed.title`, `feed.link` | The feed's own title and link. |
| `subscription.name` | The subscription's name. |
| `now`, `today` | The poll time. |

Any other reference is a config error. Every value is flattened to one line,
except in `create_if_missing.content`, where `item.summary` and `item.content`
keep their paragraphs. Because a multi-line value cannot be appended as one log
line, `then: append_section` may not use them.

In an entity body (`create_if_missing.content` and `append_section`), the text
the publisher writes is escaped: `item.id`, `item.title`, `item.authors`,
`item.categories` and `feed.title`. A title such as `![](https://t.example/p.gif)`
then shows as that literal text instead of an image, and `===` cannot turn the
line above it into a heading. Properties store the text unchanged. A bare URL in
the text may still render as a link, and the escaping adds backslashes to it.

## Authentication

```yaml
    auth:
      type: bearer        # or basic, or header
      secret: news_token  # a key in .rela/secrets.yaml
      # username: me      # basic only
      # header: X-Api-Key # header only
```

The secret is a key in `.rela/secrets.yaml`, never the value, so
`subscriptions.yaml` can be committed.
A subscription with `auth` must use an `https` URL.

## How a poll works

1. rela sends a conditional GET with the ETag and Last-Modified of the previous
   poll. A `304 Not Modified` ends the poll.
2. Redirects are followed only to the same origin, or from `http` to `https` on
   the same host. A feed cannot send the server, or its credentials, anywhere
   else. The response may be at most 10 MiB.
3. Items are sorted by publication date. The newest `max_items` are imported,
   oldest first.
4. Each item runs through the mapping. rela remembers which items it has seen
   and a hash of their content. An unchanged item is skipped without a store
   read. A changed item runs through the mapping again, so `then:` steps can
   update the entity it found.

An entity you delete is not recreated while its item stays in the feed
unchanged. If the publisher edits the item, `find` no longer matches the deleted
entity and the item is imported again.

Changing the `url` or the mapping resets this memory. The next poll looks up
every item again: it creates the entities `find` does not match, and marks the
others as seen without running their `then:` steps, because those steps already
ran under the old config. Later changes to an item run the steps as usual.

## Access control

Writes run as the principal `system:subscription:<name>`, and the audit log
records the trigger `subscription:<name>`. They pass through the normal write
path, so validation and automations apply.

In a project with `acl.yaml`, grant this principal read and write access to the
`find` and `create_if_missing` types. Without read access, `find` cannot see the
existing entities and the subscription tries to create duplicates. Without
write access, every poll fails on its first new item.

## Errors

- An item the schema rejects is logged and skipped. It is not retried until its
  content changes.
- A network error, a bad status or a malformed feed fails the run. The
  scheduler retries it with its normal backoff.
- `rela validate` checks `subscriptions.yaml` against the schema, including the
  entity types, properties and template references.

Changes to `subscriptions.yaml` take effect when the scheduler restarts.

## Trust

The feed is untrusted input. `item.summary` and `item.content` reach an entity
only after conversion to markdown, which drops scripts, styles, images and event
handlers. Other text is escaped wherever it lands in a body. Links keep only
`http` and `https` and a plain host name or IP address. The markdown punctuation
in their path, query and fragment is percent-encoded.
