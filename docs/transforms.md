# View Export & Transforms

rela can convert any markdown-producing view — an entity, a list, a Lua document —
into other formats (PDF, DOCX, ...) using external tools such as `pandoc`,
`weasyprint`, or `typst`. You register a format **once** in the metamodel and it
becomes available everywhere a view can render markdown.

## Registering transforms

Add a `transforms:` map to `metamodel.yaml`. Each entry names an external command
that converts markdown to an output format:

```yaml
transforms:
  pdf:
    from: markdown            # the only supported input format (v1)
    command: ["pandoc", "-f", "markdown", "-t", "pdf", "-o", "{out}", "{in}"]
    produces: application/pdf
  odt:
    command: ["pandoc", "-f", "markdown", "-t", "odt", "-o", "{out}", "{in}"]
    produces: application/vnd.oasis.opendocument.text
```

> **Pass the output format explicitly.** `{out}` is a temp file with no
> extension, so a converter that infers its format from the filename (pandoc
> does) needs `-t <format>` — otherwise pandoc warns "Could not deduce format
> from file extension" and silently writes HTML. If your PDF content includes
> characters the default LaTeX engine can't typeset (e.g. `★`), add
> `--pdf-engine=wkhtmltopdf` or another Unicode-capable engine.

Fields:

- **`command`** (required) — an **argv array** (not a shell string). `{in}` and
  `{out}` are substituted with temp file paths rela owns. A command that references
  neither receives its input on stdin and returns output on stdout.
- **`produces`** (required) — the output content-type, echoed into the download
  response. Validated as a well-formed media type at load.
- **`from`** (optional) — the input format; defaults to (and must be) `markdown`.

Registering `docx` above makes a "Word" export appear on every entity view and list
view automatically — no per-view wiring.

### The tool must be installed

The command's binary (e.g. `pandoc`) must be on `PATH` where rela runs. A missing
tool surfaces as a clear "not found on PATH" error, and the server logs a warning
at startup for each configured transform whose binary is absent.

## Exporting from the CLI

```bash
rela render TKT-001 --transform pdf --out ticket.pdf
```

`render` uses the built-in entity renderer (title as H1, a property table, resolved
relations, then the body) and pipes it through the named transform. It needs only
the metamodel, so it works without the data-entry server.

## Exporting from the data-entry app

Entity and list views carry an **"Export ▾"** menu populated from the registered
transforms. Choosing a format downloads the converted file.

- **Entity export:** `GET /api/v1/{plural}/{id}/_export?transform=<name>`
- **List export:** `GET /api/v1/{plural}/_export?transform=<name>&list=<listId>` —
  exports the whole filtered set (not just the current page) as a table of the list
  view's columns, capped for very large lists (a visible "showing N of M
  (truncated)" line is appended when the cap is hit).

Export is a property of an already-authorized view: it runs against the same
ACL-scoped read the view itself uses, so an export can never reveal anything the
view could not already show. Relation columns show only neighbor titles the viewer
is permitted to see.

### Custom per-entity rendering

The built-in entity renderer emits the title as an H1, the properties as a
`**name:** value` list, the resolved relations, then the body. To take full
control, give the entity type's view an **`export_render`** script:

```yaml
views:
  book:                       # keyed by entity type
    entry:
      type: book
    export_render: docs/book_card.lua
```

Now "Export as PDF/ODT" on a **book** renders through `book_card.lua` instead of
the built-in renderer — automatically, with nothing extra to pick. The script
runs in document mode (`rela.document.entry_id`, `rela.get_entity`, `rela.md.*`,
…) and its stdout is the markdown fed to the transform:

```lua
local book = rela.get_entity(rela.document.entry_id)
print("# " .. book.properties.title)
print()
print("**Year:** " .. (book.properties.year or "—"))
```

A type with no `export_render` keeps the built-in renderer. The override runs
only for an entity the caller is allowed to read — export resolves the entity
through the ACL gate first, so a denied caller gets a 404 and the script never
runs.

## Security notes

- Transform commands come from project config (`metamodel.yaml`) — the same trust
  level as attachment scan/transform commands and schedules. A request may only
  choose a registered transform **name**; it can never supply a command, flag, or
  path.
- Commands run as an **argv array with no shell** (`{in}`/`{out}` are rela-chosen
  temp paths), under a timeout and an output-size cap.
- Exported downloads are served defensively (forced download, `nosniff`, a
  sandboxing CSP, `no-store`) because the produced bytes embed user content.

## Not yet supported (v1 limits)

- Format chaining (e.g. markdown → HTML → PDF) — each transform is a single step.
- A render override for LIST export — `export_render` applies to entity export
  only; list export always emits the column table.
- Asynchronous export for slow converters (LaTeX / LibreOffice) — exports run
  synchronously within the request.
- A per-view configurable row cap for list export.
