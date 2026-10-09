# Basecamp to-do sync

This example keeps rela `todo` entities and every Basecamp 4 to-do the
account can see in step, in both directions. It also mirrors the Basecamp
projects and to-do lists as `project` and `todolist` entities, and links
each todo to its list and each list to its project. It uses the sync API
described in the Lua scripting guide ("Sync connectors"), `rela.md` to
convert between Basecamp's HTML and rela's markdown, and `rela.oauth` for
the access token.

| File               | Goes to                      | What it does                                         |
| ------------------ | ---------------------------- | ---------------------------------------------------- |
| `basecamp.lua`     | `scripts/basecamp.lua`       | The connector: pulls when scheduled, pushes on save. |
| `schema.yaml`      | merge into `schema.yaml`     | The types, the links and the push automation.        |
| `schedules.yaml`   | merge into `schedules.yaml`  | The pull, every 5 minutes.                           |
| `acl.yaml`         | merge into `acl.yaml`        | What the connector identity may do.                  |
| `connections.yaml` | `connections.yaml`           | How rela refreshes the Basecamp token.               |
| `consent.py`       | run once                     | Gets the first refresh token.                        |

One script does both directions because a rela script cannot load another
file. When the global `entity` is set (the automation), it pushes that todo.
Otherwise (the schedule), it pulls everything.

## Where it runs

Tokens are stored by rela, never in a script. Where they are stored depends
on how you run rela:

| Setup                    | Token store                                     | Push runs                          |
| ------------------------ | ----------------------------------------------- | ---------------------------------- |
| `rela-server-sqlite`     | `rela.db`, sealed with the `token_key` secret   | on the job queue, after the save   |
| `rela-server-postgres`   | the database, sealed with `token_key`; one refresh at a time across all servers | on the job queue (durable)         |
| desktop app              | the OS keychain                                 | on the job queue, after the save   |
| `rela` CLI (sqlite, pg)  | as the server on that backend                   | in the foreground, after each save |
| file or memory backend   | none: `rela.oauth` reports `not_configured`     | not usable                         |

The desktop app keeps tokens only in the keychain, and the CLI and
server keep them only in `rela.db` or the database. A token stored in one
is not seen by the other. When you move the project between them, store
the token again.

On sqlite and the file-backed queue, jobs still waiting when the process
stops are lost. The next pull merges what they would have pushed, so a lost
push is late, not gone.

## Setup

1. Register an application at <https://launchpad.37signals.com/integrations>.
   Note its client id, client secret and redirect URI.
2. Add the secrets to `.rela/secrets.yaml` (or the desktop app's Secrets
   section):

   ```yaml
   token_key: <32 random bytes, base64>   # not needed on the desktop app
   basecamp_client_id: ...
   basecamp_client_secret: ...
   basecamp_account_id: "1234567"
   basecamp_todolist_id: "7654321"   # optional; see "New todos" below
   ```

   `openssl rand -base64 32` makes a `token_key`. Keep a copy: tokens sealed
   with a lost key cannot be read, and `rela token status` says so.
3. Copy and merge the files as the table above says. Put your own contact
   address in the User-Agent, in both `connections.yaml` and
   `basecamp.lua`; Basecamp refuses requests without one.
4. Get the first token. Register `http://localhost:8765/callback` as the
   app's redirect URI (or pass yours with `--redirect-uri`; it must point at
   localhost), then run:

   ```sh
   BASECAMP_CLIENT_ID=... ./consent.py | rela token set basecamp
   ```

   The script asks for the client secret without showing it. Do not put
   the secret on the command line, where your shell history keeps it. It
   opens Basecamp in your browser and runs a small web server on the
   redirect URI that catches the answer, so you copy nothing by hand. It
   lists the Basecamp accounts the token reaches; use one of those ids as
   `basecamp_account_id`. Pass your User-Agent with `--user-agent` or
   `BASECAMP_USER_AGENT`. On SQLite, stop `rela-server` first: the CLI
   cannot open the database while the server holds it.

   **Desktop app:** run `./consent.py` without the pipe. It prints a line of
   JSON. In the app, open Settings, Connections, pick `basecamp`, paste the
   line and choose Save Token.
5. Check it: `rela token status basecamp` says `ok`.

## How it behaves

- **Identity.** Both directions run as `integration:basecamp`. That name
  holds only the `basecamp-connector` role from `acl.yaml`, so the connector
  can read and write todos and move the `sync/basecamp` tags, and nothing
  else. No person can sign in under an `integration:` name.
- **What is pulled.** Every active to-do and to-do list in every project
  the account can see, read through Basecamp's recordings listing. A to-do
  becomes a todo with a `basecamp` reference, linked with `in-list` to its
  list. A list becomes a `todolist`, linked with `in-project` to a
  `project`. Archived and trashed projects are not read.
- **Projects and lists belong to Basecamp.** The pull creates and renames
  them, and relinks a to-do that moved to another list. A title or link
  changed in rela is overwritten on the next pull. Nothing is pushed for
  them.
- **New todos.** A todo created in rela becomes a Basecamp to-do on its
  first save. It goes to the list its `in-list` link names, or else to
  `basecamp_todolist_id`. With neither, the todo gets a `sync_conflict`
  asking for a list; add the link and save it again.
- **Bodies.** Basecamp stores a to-do's description as HTML; rela stores
  markdown. The connector converts both ways with `rela.md.from_html` and
  `rela.md.to_html`. Lists, headings, emphasis, code, quotes, tables and
  links survive; colors do not. The first pull after a body changes may
  rewrite it once into the form both sides agree on.
- **Bodies rela cannot hold.** Attachments, mentions and images in a
  Basecamp description are dropped from the rela copy. To keep them in
  Basecamp, a local edit to such a body is not pushed: the todo gets a
  `sync_conflict` starting with `body:`. Edit that body in Basecamp, or undo
  the local edit. The same holds the other way for a rela body with an image
  or raw HTML.
- **Edits.** Each side's change since the last agreed state is carried to
  the other. A save that changed no synced field makes no request to
  Basecamp at all.
- **Conflicts.** When both sides changed the same field, nothing is written
  on either side. The todo's `sync_conflict` says which fields. Make the two
  sides agree, and the next pull clears the note.
- **Retried pushes.** A new todo is linked to its Basecamp to-do right
  after the create request, before anything else can fail. So when the job
  queue retries a failed push, it never creates a second to-do. If marking
  the new to-do complete failed, the next pull reports `done` as a
  conflict; complete it in Basecamp to clear it.
- **Deleted or archived to-dos.** A todo whose Basecamp to-do is no longer
  listed gets a `sync_conflict` starting with `vanished:`. This includes the
  to-dos of a project that was archived. The connector never deletes
  anything.
- **Fields rela does not own.** A Basecamp update replaces the whole to-do,
  so the push reads it first and sends back the assignees, completion
  subscribers and start date unchanged.
- **Rate limits.** A pull that gets a 429 stops before it writes anything,
  and moves no tag; the next run starts over. A push that gets a 429 fails,
  and the job queue retries it later.
- **Expired or revoked access.** rela refreshes the access token when it
  expires. When Basecamp rejects one anyway, the script invalidates it and
  tries once more with a new one. When the refresh token itself is refused,
  `rela token status` reports `needs_consent`: run `consent.py` again.

### The lost-update window

The push reads the Basecamp to-do, merges, then reads it once more and
compares `updated_at` just before writing. If someone edits the to-do in
Basecamp in the moment between that second read and the write, the write
replaces their edit. Basecamp has no conditional update, so no client can
close this gap. It is a fraction of a second per push.

### Nested pushes during a pull

When the pull writes a todo, that write is a save, so the push automation
starts for it. On the job queue it runs after the pull and finds nothing to
send. In the CLI it runs at once, inside the pull, before the tag has
moved; it may then read the to-do from Basecamp once and stop without
writing.
