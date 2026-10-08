# Desktop documents: what lives where

Status: built. The section "End state" lists what is left. This note records where the desktop app is heading for `.rela`
documents, so later work moves toward it rather than around it.

## Goal

A `.rela` document is one file that holds a whole project: the config, the
data, attachments, comments, history and app settings. Before this work, a few
things lived in a private folder per document, under the user's config
directory, keyed by a hash of the document's path:

| What                 | Why it was outside                           |
| -------------------- | -------------------------------------------- |
| `.rela/secrets.yaml` | Secrets must never be stored in the database |
| `.rela/mail.yaml`    | Read from disk only                          |
| `.rela/ai.yaml`      | Read from disk only                          |
| `.rela/audit/`       | The audit sink writes JSONL files            |
| `.rela/search/`      | The sqlite build paired SQLite with bleve    |

Because the folder is keyed by path, moving or renaming a document lost its
secrets and rebuilt its index. The direction below removes every reason for
the folder, so a document depends on nothing but itself and the user's
keychain.

## Direction

### Secrets go in the OS keychain

On the desktop, secrets are kept in the operating system's credential store,
through `github.com/zalando/go-keyring`:

- macOS: the login Keychain, through `/usr/bin/security`.
- Windows: Credential Manager.
- Linux: the Secret Service (GNOME Keyring, KWallet).

How it is stored:

- **One item per secret**, not the whole file in one item. Windows limits a
  secret to 2560 bytes, and macOS limits the whole command to about 3000
  bytes.
- **Keyed by a document ID**, a random ID stored inside the document when it
  is created. Not by path, so the secrets are not lost when a document moves.
- **An index item** lists the secret names for a document, because go-keyring
  has no way to list items. It also lists the places allowed to read them, as
  hashes of each project's `.rela` directory.
- **Released per place, not per ID.** The ID comes from the document, so a
  document from someone else can carry the ID of one of the user's own
  documents (copied from one the user shared). Its scripts get the keychain
  secrets only at a place where the user set a secret or approved them in
  Project Settings. A moved document therefore asks once. An ID that is not 32
  lowercase hex characters is replaced, because it is part of keychain
  account names.
- **A host config source, chosen at wiring.** Secrets and the AI and mail
  settings are read through one interface (`lua.HostConfig`, with
  `hostconfig.Dir` as the `.rela` implementation). `appbuild.WithHostConfig`
  replaces it; only the desktop does, with a source over the keychain and
  the project's state (`cmd/rela-desktop/hostconfig.go`). The CLI and
  `rela-server` keep the files.
- **The file stays as the fallback**, for Linux without a Secret Service and
  for existing projects. A keychain secret wins over a file secret with the
  same name. Per-script overrides stay a `secrets.yaml` feature.
- **File > Project Settings** adds, replaces and removes secrets. The window
  receives secret names only; a value goes into the keychain and is never
  sent back to a page.

What this protects, and what it does not:

- It keeps secrets out of files, backups, synced folders and git, and
  encrypted at rest.
- It does **not** stop other programs running as the same user. go-keyring
  creates macOS items through `/usr/bin/security`, so that tool, not rela, is
  the trusted app, and any process can read the item through it without a
  prompt. That is about as strong as a `0600` file. Per-app access control
  would need the native Keychain API through cgo and a signed build; we are
  not doing that now.

A copy of a document carries the same document ID, so two copies on one
machine share secrets. A copy sent to another machine finds no secrets there.
Both are acceptable.

### The audit log is a no-op on the desktop

The desktop wires `audit.Nop{}` instead of the filesystem sink.

Nothing reads the audit log: it is written for operators of a shared server
to answer "who changed what". On the desktop there is one user, and the
document's own version history (in the database) already answers that.
Writing JSONL files beside a document adds a folder and nothing a user can see.

The CLI and `rela-server` keep their audit sinks. Operations that write an
explicit audit record (data import, migrations) keep doing so through
whichever sink they are given.

### AI and mail settings go in the document

`ai.yaml` and `mail.yaml` hold no secrets. `ai.yaml` names an environment
variable for the API key (`api_key_env`), and the SMTP password is already in
the secrets store. What remains (provider, model, base URL, SMTP host) can
travel with the document.

Store them in the project's state (`state.KV`, inside `rela.db` for a
document), under `desktop/hostconfig/`. With the API key and SMTP password in
the keychain, nothing in them is secret. File > Project Settings edits them
and checks they parse before saving. A `.rela/ai.yaml` or `.rela/mail.yaml`
is still read when the state has none.

An environment variable is also the wrong source for a key on the desktop:
an app started from Finder or the Dock does not see the user's shell
environment. The AI provider now reads its key from the secret `ai_api_key`
first, on every build, and falls back to `api_key_env`.

Storing them as project config (`project_files`) instead would make them part
of the project for every build. That was not done: they stay settings of one
installation, as `.rela/` made them.

### Search uses SQLite FTS5

The sqlite build searches with FTS5 in the same database (DEC-10Z731), as the
postgres build searches inside its own. The index travels with the document,
and no index folder is needed.

- The index is the `entity_search` table, with the `trigram` tokenizer. It
  answers case-insensitive substring queries over the same text postgres
  matches (id, string properties, body), so the two database builds find the
  same entities. modernc SQLite has no API for custom tokenizers (DEC-LFSYNY
  trade-off 4), and `trigram` is the built-in one with these semantics.
- Triggers on `entities` keep the index current inside each write
  transaction, on every write path. There is no backfill and no rebuild
  after a crash. Schema version 12 adds the index.
- An index row is keyed by a row in `entity_search_key`, which maps an
  entity's `(id, face)` to an `INTEGER PRIMARY KEY`. The `entities` rowid
  is not a usable key: the table has no `INTEGER PRIMARY KEY`, so `VACUUM`
  may renumber it. Search joins through the key table by `(id, face)`.
  Schema version 13 rebuilds a version 12 index, which used the rowid.
- `search.Visible` wraps the searcher, so the ACL contract does not change.
  It passes the store conformance search suites.
- The index makes the file larger, roughly by the size of the indexed text
  plus its trigrams.

## End state

With all four in place, a document needs nothing from its private folder:

- the document file holds config, data, history, settings and the search
  index;
- the keychain holds secrets, keyed by the document's ID;
- SQLite's own `-wal`, `-shm` and `.lock` files sit beside the document while
  it is open.

`project.Context` still needs a root, so the folder still exists, empty
unless an older document left files in it. Changing the lookup so a document
needs no root at all is the remaining step.

## Order of work

1. Audit no-op on the desktop. Small, removes one folder.
2. Document ID in the database, keychain secrets source, and the secrets
   screen. Fixes the moved-document problem.
3. AI and mail settings in `state.KV`, AI key in the keychain.
4. FTS5 searcher, after its own decision record.
