# Basecamp to-do sync

This example keeps rela `todo` entities and the to-dos of one Basecamp 4
to-do list in step, in both directions. It uses the sync API described in
the Lua scripting guide ("Sync connectors") and `rela.oauth` for the access
token.

| File               | Goes to                      | What it does                                         |
| ------------------ | ---------------------------- | ---------------------------------------------------- |
| `basecamp.lua`     | `scripts/basecamp.lua`       | The connector: pulls when scheduled, pushes on save. |
| `schema.yaml`      | merge into `schema.yaml`     | The `todo` type and the push automation.             |
| `schedules.yaml`   | merge into `schedules.yaml`  | The pull, every 5 minutes.                           |
| `acl.yaml`         | merge into `acl.yaml`        | What the connector identity may do.                  |
| `connections.yaml` | `connections.yaml`           | How rela refreshes the Basecamp token.               |
| `consent.sh`       | run once                     | Gets the first refresh token.                        |

One script does both directions because a rela script cannot load another
file. When the global `entity` is set (the automation), it pushes that todo.
Otherwise (the schedule), it pulls the whole list.

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
   basecamp_todolist_id: "7654321"
   ```

   `openssl rand -base64 32` makes a `token_key`. Keep a copy: tokens sealed
   with a lost key cannot be read, and `rela token status` says so.
3. Copy and merge the files as the table above says. Put your own contact
   address in the User-Agent, in both `connections.yaml` and
   `basecamp.lua`; Basecamp refuses requests without one.
4. Get the first token:

   ```sh
   BASECAMP_CLIENT_ID=... BASECAMP_REDIRECT_URI=https://example.com/callback \
     ./consent.sh | rela token set basecamp
   ```

   The script asks for the client secret without showing it. Do not put
   the secret on the command line, where your shell history keeps it. The
   script lists the Basecamp accounts the token reaches; use one of
   those ids as `basecamp_account_id`. On SQLite, stop `rela-server` first:
   the CLI cannot open the database while the server holds it.

   **Desktop app:** run `./consent.sh` without the pipe. It prints a line of
   JSON. In the app, open Settings, Connections, pick `basecamp`, paste the
   line and choose Save Token.
5. Check it: `rela token status basecamp` says `ok`.

## How it behaves

- **Identity.** Both directions run as `integration:basecamp`. That name
  holds only the `basecamp-connector` role from `acl.yaml`, so the connector
  can read and write todos and move the `sync/basecamp` tags, and nothing
  else. No person can sign in under an `integration:` name.
- **First sync.** A to-do in Basecamp becomes a todo in rela with a
  `basecamp` reference. A todo created in rela becomes a Basecamp to-do on
  its first save.
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
- **Deleted, archived or moved to-dos.** A todo whose Basecamp to-do is no
  longer in the list gets a `sync_conflict` starting with `vanished:`. The
  connector never deletes anything. The `basecamp` reference URL ends in
  `#todolist-<id>`, naming the list the to-do was synced from. Only todos
  of the configured list are checked, so pointing `basecamp_todolist_id`
  at another list does not mark the old list's todos as vanished.
- **Fields rela does not own.** A Basecamp update replaces the whole to-do,
  so the push reads it first and sends back the assignees, completion
  subscribers and start date unchanged.
- **Rate limits.** A pull that gets a 429 stops before it writes anything,
  and moves no tag; the next run starts over. A push that gets a 429 fails,
  and the job queue retries it later.
- **Expired or revoked access.** rela refreshes the access token when it
  expires. When Basecamp rejects one anyway, the script invalidates it and
  tries once more with a new one. When the refresh token itself is refused,
  `rela token status` reports `needs_consent`: run `consent.sh` again.

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
