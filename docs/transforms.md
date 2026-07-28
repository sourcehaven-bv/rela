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

### Custom per-list rendering

List export has the same escape hatch, configured on the **list** rather than
the entity type — so two lists of the same type can export differently, and the
list that owns the columns and filters also owns its export:

```yaml
lists:
  tickets:
    entity_type: ticket
    columns: [...]
    export_render: docs/ticket_report.lua
```

Now "Export as PDF/ODT" on the **tickets** list renders through
`ticket_report.lua` instead of the built-in column table. The script receives
the rows the server already resolved, plus the resolved query as read-only
context:

```lua
print("# " .. rela.document.list_id .. " report")
if rela.document.query.filters.status then
  print("_status: " .. rela.document.query.filters.status .. "_")
end
print()

for _, row in rela.document.rows() do
  print("## " .. row.properties.title)
  print(row.content or "")
end

if rela.document.truncated then
  print(("\n_Showing %d of %d._"):format(rela.document.count, rela.document.total))
end
```

What a list render sees on `rela.document`:

| Field | Meaning |
|---|---|
| `list_id`, `entity_type` | which list, over which type |
| `rows()` | iterator over the rows — `for _, row in rela.document.rows() do` |
| `row(i)` | one row by 1-based index, or nil |
| `count` | rows this render can see (post-cap) |
| `total`, `truncated` | rows before the cap, and whether it applied |
| `query.q`, `query.filters`, `query.sort` | the resolved request, read-only |
| `entry_id` | **absent** — a list render has no entry entity |

Two contracts worth knowing:

- **Render the rows you are given.** `rela.document.rows()` is exactly the
  ACL-scoped, filtered, sorted, capped set the on-screen list resolved. Building
  your own set with `rela.list_entities` would silently ignore the filters the
  user actually had applied and escape the row cap, so an export would stop
  matching the view it came from. Narrowing, grouping, or sorting the given rows
  in Lua is fine — that is the intended way to shape the output.
- **Rows are materialized lazily**, one at a time, so a large export stays flat
  in memory. Each call to `rows()` starts a fresh walk, so you may iterate more
  than once (to total, then to emit); each walk re-materializes, which costs CPU
  rather than memory. Use `count` and `row(i)` when you only need a length or
  one row.

A list with no `export_render` keeps the built-in column table. As with entity
export, the override is config-selected — a request may choose a transform
*name*, never a renderer.

## Security

### The threat

Export feeds **attacker-influenceable content** — an entity body is free markdown
writable by anyone with write access — into a **third-party document converter**
running on your server. That is a real attack surface, not a theoretical one:

- **SSRF (verified).** Converters fetch remote resources by design. A body
  containing `![x](http://169.254.169.254/…)` makes the *server* issue that
  request, from its own network position — cloud metadata, internal services.
  Pandoc's own manual documents this for HTML input.
- **Local file disclosure (verified).** A body carrying a raw LaTeX block —
  ` ```{=latex} \input{/etc/passwd} ``` ` — makes the TeX engine read that file
  and **embed its contents into the exported PDF**, which then lands with
  whoever downloaded it. No network required. `pandoc --sandbox` does not stop
  it; its manual exempts PDF production.
- **Parser RCE.** Converters are large C/C++/TeX codebases with CVE history.
  `wkhtmltopdf` is **unmaintained** and has a known unpatched file-read whose
  `--disable-local-file-access` mitigation is bypassable — avoid it.
- **Resource exhaustion.** A crafted document can drive a converter to consume
  unbounded memory, fork endlessly, or fill the disk.

This matters most for **network deployments**, where a remote user with write
access can reach the server's network. For purely local CLI use the bar is
lower — someone who can run `rela` can already run anything.

### What rela does about it

Commands run **confined**, and this is enforced in one shared place
(`internal/cmdexec`) so attachment processing gets the identical guarantees:

| Control | Mechanism |
|---|---|
| No network | Linux: bubblewrap (`--unshare-all`). macOS: `sandbox-exec` (`deny network*`) |
| Writes | only the run's own temp dir |
| Reads (**Linux only**) | an allowlist of binaries/libraries/fonts — the project directory, `/etc`, `/home` and `/root` are simply not present |
| Memory / processes / file size / CPU | `RLIMIT_AS` / `NPROC` / `FSIZE` / `CPU` (Linux) |
| No orphaned helpers | the whole process group is killed at the deadline |
| Concurrency | a bounded pool caps simultaneous conversions |
| Wall clock + output size | per-command timeout and cap |

**Fail closed.** Where no sandbox mechanism exists (Windows, BSD, or a Linux
kernel without unprivileged user namespaces), commands **refuse to run** rather
than run unconfined. This blocks command execution only — the server still starts
and everything that doesn't shell out keeps working. An operator who provides
isolation another way (a locked-down container, a no-egress network policy) can
explicitly accept unconfined execution. The startup log states the posture.

> **Converter flags are not a substitute.** `pandoc --sandbox` restricts pandoc's
> own file access but explicitly **does not cover PDF production** — the PDF
> engine is a separate process outside it. Verified: with `--sandbox`, a
> markdown-to-PDF run still performed the outbound fetch *and* still read a local
> file via `\input`. Use it as defence-in-depth, not as the control.

**macOS does not confine reads.** Write and network restrictions work there, but
the read allowlist is Linux-only: `sandbox-exec`'s profile language is
undocumented and deprecated, and a read-restricting profile could not be made to
behave consistently. On macOS a crafted document can therefore still disclose
server-readable files into an export. Treat macOS as a development tier and run
untrusted content through the Linux tier.

### Access control

- **Entity-level ACL**: export sits downstream of the same read gate as the
  entity view — an entity the caller may not read is an indistinguishable 404,
  and relation groups / relation columns only ever show visible neighbors.
- **Field-level redaction**: exports are field-redacted like every other
  read-out surface. The markdown handed to a converter is built from a
  redacted copy of the entity (`internal/visibility`, DEC-ZBI39P): a property
  hidden from the caller by a `visible:` policy appears in no exported cell,
  heading, or filename — a hidden display property falls back to the entity
  ID, including for visible neighbors whose titles are hidden.
- **`export_render:` override scripts** run under the **caller's principal**
  (`rela.principal` reflects the requesting user, and the render is
  attributable/cancellable), and since TKT-ZF2DTV their *own* reads are
  ACL-bound too: `rela.get_entity`, `rela.list_entities`, `rela.search`,
  `rela.get_relations` and the trace bindings all return the caller's view —
  hidden entities absent, hidden properties redacted. An override script
  therefore cannot widen an export past what the requester may see.
  - `rela.get_relations` is **peer-gated**: a relation appears only when
    *both* endpoints are visible, so an empty result means "none you may
    see", not "no such edges".
  - `rela.update_entity`'s read-before-write is deliberately **not**
    redacted — reading a redacted copy there would erase the caller's hidden
    properties on save. Writes remain gated by the ACL as before.
  - A **list** override (`lists.<id>.export_render`) receives its rows from
    the server, already row-gated and field-redacted by the same read the
    on-screen list uses — the script never queries for them. That is why an
    export always matches the view it came from, and why the row cap still
    bounds it.

### Other notes

- Transform commands come from project config (`metamodel.yaml`) — the same trust
  level as attachment scan/transform commands and schedules. A request may only
  choose a registered transform **name**; it can never supply a command, flag, or
  path.
- Commands run as an **argv array with no shell** (`{in}`/`{out}` are rela-chosen
  temp paths).
- Exported downloads are served defensively (forced download, `nosniff`, a
  sandboxing CSP, `no-store`) because the produced bytes embed user content.
- Prefer `-f commonmark_x` over `-f markdown` for untrusted input: pandoc's
  manual notes it is far less prone to pathological parsing performance.

## Not yet supported (v1 limits)

- Format chaining (e.g. markdown → HTML → PDF) — each transform is a single step.
- Asynchronous export for slow converters (LaTeX / LibreOffice) — exports run
  synchronously within the request.
- A per-view configurable row cap for list export.
