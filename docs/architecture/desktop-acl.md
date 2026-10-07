# Desktop access control: confining what acts for the user

Status: direction. Nothing here is built yet. This note records where the
desktop app is heading for access control, so later work moves toward it
rather than around it. The decision is DEC-QNZSC6.

## Goal

The desktop app wires `acl.NopACL{}` (`cmd/rela-desktop/hostconfig.go`). The
reason still holds: the user owns the machine and the files, so an ACL could
not restrict them.

What changes is that more things now act **for** the user without the user
doing each action:

- the scheduler, running a document's scripts, possibly with AI;
- an MCP client such as Claude Code, editing the graph;
- later, deep links, Shortcuts and Siri intents, and a Spotlight indexer.

Each of these can do damage by mistake (an AI hallucinating) or because
content it read told it to (prompt injection). The ACL's job on the desktop is
to limit them. It is not there to limit the user.

## Principals

Every entry point acts as its own principal, carried by `principal.Principal`:
the OS user, plus a tool and a client name.

| Principal                    | Default role | Default capabilities |
| ---------------------------- | ------------ | -------------------- |
| Window                       | `owner`      | all                  |
| Scheduler                    | none         | none                 |
| MCP client (one per pairing) | none         | none                 |
| Notifications and badge      | `reader`     | none                 |
| Deep link (later)            | `navigate`   | none                 |
| Intent (later)               | none         | none                 |

"None" means the principal can do nothing until the user grants it something.
A document that worked before this change keeps working for its windows; its
schedules need a grant once.

## Who decides what

The ACL has two parts, and they have different owners.

| Part                                            | Owner                  | Stored in                   |
| ----------------------------------------------- | ---------------------- | --------------------------- |
| Role definitions: what a role may read, write   | The document's author  | `acl.yaml` in the document  |
| Role assignment: which principal has which role | The desktop app's user | App settings on the machine |
| Capabilities: AI, mail, HTTP, commands, secrets | The desktop app's user | App settings on the machine |

- **The window user is always `owner`.** The assignment part of a document's
  `acl.yaml` is ignored on the desktop.
- **Assignments never live in the document.** `state.KV` is inside `rela.db`,
  so storing grants there would let a document arrive with its own automation
  already trusted. They live in app settings, next to the keychain index.
- **Grants are keyed by document ID and place**, like secrets
  (`desktop-documents.md`). A copy carries the original's ID; keyed by ID
  alone, it would inherit the grants the user gave the original. A moved
  document asks again; the settings window offers to copy the grants from
  the document's earlier place.

### Built-in roles

Most documents have no `acl.yaml`, and the user still needs something to
assign. The desktop provides:

- `owner`: everything. Used by the window only.
- `reader`: read every entity and relation; no writes.
- `navigate`: resolve a route only.

Roles from the document's `acl.yaml` are offered next to these. A document's
role is only as honest as its author: a hostile document can define a role
named `viewer` that may delete everything. So the settings window shows what a
document role actually allows (types it can read, create, update, delete),
computed by the ACL, and the user approves that effect rather than a name.

## Two kinds of limit

### Graph writes: the ACL

A principal's role limits which types it may create or update, which fields it
may change, and whether it may delete or rename. `entitymanager` already
enforces this on every write.

The limit holds through cascades. An automation triggered by an MCP write runs
under the MCP client's ceiling, not the owner's.

The window's `owner` role takes a fast path that skips the visibility
wrappers, so enabling the ACL costs the common case nothing.

### Side effects: capability bundles

The ACL covers entities and relations only. A script limited to reading can
still send mail, call a web service, call the AI, run a command or read a
secret. Those are the dangerous parts, so they are granted separately, as
capabilities:

- `ai`
- `mail`
- `http`
- `commands`
- `secret:<name>`, per named secret

They are enforced the way the project already prefers: a capability bundle,
not a check. The Lua `ReadDeps` / `WriteDeps` are built per principal and
leave out what was not granted. A script without `http` has no `http`
binding. A missing binding cannot be bypassed the way a forgotten check can.

### The combination to warn about

A principal is most dangerous when it has all three of:

1. untrusted input (entity text, mail, web pages);
2. broad read access;
3. a way out (HTTP, mail, or AI calls to a third party).

Injected text can then send everything it can read somewhere else. The
settings window warns when one assignment combines all three.

## MCP on the desktop

### One server, inside the app

A `.rela` document is open in one process at a time: the sqlite backend holds
an exclusive lock. An MCP server that opened the file itself would be refused.
So the app hosts the MCP server, over the documents it has loaded, and a
small bridge connects MCP clients to it:

```text
Claude Code, Claude Desktop, ...
  │ stdio
  ▼
rela-desktop mcp        bridge that MCP clients launch
  │ Unix socket (0600) in the app's support folder; a named pipe on Windows
  ▼
Rela Desktop            one MCP server over all open documents
```

- MCP clients launch a command over stdio, so the bridge is what goes in the
  client's config.
- A socket, not a localhost HTTP port: no browser can reach it, and no port is
  exposed.
- If the app is not running, the bridge says so (or starts it).
- Writes arrive in the app's own process, so open windows update live through
  the existing change events.

### Tools

- `list_documents` returns the documents this client may use: ID, name, and
  whether it is open. It is filtered by the client's grants; a document
  without a grant does not appear.
- Every existing tool (`list_entities`, `show_entity`, `create_entity`,
  `schema`, ...) takes a `document` argument. It is explicit on every call,
  not a stateful "use document X", so an agent working with two documents
  cannot write to the wrong one after losing track of state.
- The server's instructions cannot list entity types, because they differ per
  document; they point to `schema(document)`.

`internal/mcp` stays per project. The meta server routes each call to the
matching project's instance and attaches the client's principal.

### Pairing and grants

- The first connection from a client shows a native dialog: "Claude Code
  wants to connect to Rela Desktop". Page script cannot answer it.
- The client gets an identity that is its principal (tool `mcp`, client name).
- The user grants it documents, a role and capabilities in the settings
  window, keyed by document and place as above.

### Open documents only, at first

The first version serves only documents open in the app:

- the user can see what an agent can touch;
- nothing opens or locks a file in the background;
- the scheduler rule stays as it is: it runs only while a window is open.

Opening closed documents in the background can follow, as its own grant.

### Project folders

For a folder, `rela mcp` from the CLI works without the app, because a folder
has no exclusive lock. The meta server also serves folders open in the app,
so one client config covers both. The CLI server stays for headless use.

## Seeing and undoing what an agent did

- Each write records its principal in version history (`last_edited_by_tool`).
- A window shows activity, for example "Claude Code changed 3 entities".
- Undo by principal: "revert everything the scheduler changed since 09:00" is
  a query over history the backend already keeps.
- A proposal mode, where an agent writes to a draft face and the user accepts,
  is on the backlog separately.

## What this does not protect against

Any process running as the same user can run the bridge and claim to be any
paired client, and can read the app's settings. This confines agents that mean
well and make mistakes. It does not stop malware running as the user; nothing
short of a signed app with per-app keychain access would, and that is the same
limit the keychain secrets have.

## Order of work

1. A distinct principal per entry point (scheduler, notifications), recorded
   in version history. Cheap; gives attribution and undo by principal.
2. Capability bundles per principal: `ai`, `mail`, `http`, `commands`,
   secrets. Closes the largest gap.
3. The declarative ACL on the desktop: window as `owner` with a fast path,
   built-in roles, assignments in app settings keyed by document and place.
4. MCP: the in-app server, the bridge, pairing, `list_documents`, grants.
5. Undo by principal, and the activity indicator.
