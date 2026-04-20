<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# At-Rest Encryption

rela can transparently seal every entity, relation, and attachment file in your
project so that the on-disk contents are unreadable without an authorized
private key. Encryption uses [age](https://age-encryption.org/) under the hood:
no custom crypto, one data key per file, post-quantum hybrid recipients
(ML-KEM-768 + X25519).

## When to use it

Turn encryption on when:

- The project contains sensitive design material and may end up on a stolen
  laptop, a shared filesystem, or a backup that escapes your control.
- You want to commit the repo to a remote (GitHub, GitLab) or sync it via
  Dropbox / iCloud while keeping the contents private to a known set of
  team members.
- Your threat model is "adversary with disk read access but no private key."

Leave it off if the data is not sensitive or if you need `git diff`-style
readable history on the raw files.

## Mental model

| File                              | Sealed? | Notes                                       |
| --------------------------------- | ------- | ------------------------------------------- |
| `entities/<type>/*.md`            | yes     | Full file (frontmatter + body)              |
| `relations/*.md`                  | yes     | Relation filename stays cleartext           |
| `attachments/**/*`                | yes     | Payload and metadata sidecars both sealed   |
| `.rela/fsstore-index.json`        | yes     | Derived cache — would leak property values  |
| `recipients.age`                  | sealed  | Authoritative recipient list (see below)    |
| `metamodel.yaml`                  | no      | Bootstrap config; tooling needs it readable |
| `groups.yaml`, `schedules.yaml`   | no      | Bootstrap config                            |
| `templates/`                      | no      | Project scaffolding                         |
| `.rela/repo-id`                   | no      | Per-repo fingerprint (32 hex chars)         |
| `.rela/cache.json`                | no      | Graph cache (rebuildable)                   |
| `.rela/ai.yaml`, `.rela/secrets.yaml` | no  | Project-scoped tooling config               |

### User-local state lives outside the project tree

Per-user state — age private key, rendered-document cache, UI state,
user defaults, palette overrides, scheduler execution history, and the
encryption rollback-defense marker — lives **outside** the project
directory under the OS user-config tree:

| Platform | Path |
| -------- | ---- |
| Linux / BSD | `$XDG_CONFIG_HOME/rela/repos/<repo-id>/` (default `~/.config/rela/repos/<repo-id>/`) |
| macOS | `~/Library/Application Support/rela/repos/<repo-id>/` |
| Windows | `%AppData%\rela\repos\<repo-id>\` |

`<repo-id>` is the UUID-shaped fingerprint stored in `.rela/repo-id`
(auto-generated on first use). For encrypted repos, `.rela/repo-id`
is cross-checked against the RepoID baked into `recipients.age`;
a mismatch aborts with a clear error — the signature of a `.rela/`
directory copied in from another project.

Use `$RELA_USER_STATE_DIR` to override the base path (absolute only).
Rela refuses overrides that point inside the project tree — the whole
point of this layout is to keep state out of any synced directory.

This means it is **safe** to put your project directory on Dropbox,
iCloud Drive, or OneDrive: nothing in the project tree is user-local
plaintext that could leak via a sync. The project contains only
committed source and the `.rela/repo-id` fingerprint.

### Filenames and sizes leak

Filenames, directory structure, and file sizes are visible to anyone
with disk read access. If a filename would leak something sensitive
(e.g., `REQ-project-terminus.md`), rename before enabling encryption.

Relation filenames are particularly telling: `REQ-001--blocks--DEC-042.md`
reveals the graph structure even when the contents are sealed.
Acknowledged limitation; cannot be hidden without giving up the
content-addressable filesystem layout.

## Key management

### Where keys live

Recipients — the authoritative list of who can decrypt the repo — live
in an encrypted file at `<repo>/recipients.age`. This file is checked
into git. Its contents (recipient names + age public keys + monotonic
version + per-repo UUID) are sealed under those same recipients, so
**only someone who can already read the repo can add another
recipient**. The cloud adversary we defend against lacks any private
key and therefore cannot silently expand the recipient set.

The local private identity is resolved in this order:

1. `$RELA_KEY_FILE` — explicit path via environment variable.
2. The `key` file inside the per-user, per-repo state directory
   (see "User-local state" above).

There is no project-tree fallback. A key inside the repo tree would
defeat the at-rest encryption threat model (cloud-synced project =
cloud-synced key). `rela keys init --identity <path>` installs the
provided key into the user-state directory at the standard location;
the original source file is not moved.

The private key file is in the standard `AGE-SECRET-KEY-PQ-1...`
format (hybrid post-quantum; single line).

### Rotation and versioning

Every `rela keys add` / `rela keys remove` bumps a monotonic version
counter stored in `recipients.age` and stamped into every re-sealed
data file. The per-user, per-repo state directory
(see "User-local state") records the highest version this machine
has seen. On every read, rela verifies the file's
version is not lower than the last-seen version — catching attempts
by a cloud-side adversary to restore an older snapshot of any single
sealed file.

First read on a new machine is TOFU (trust on first use): there's no
prior state to compare against, so rela accepts whatever version it
sees and records it. Subsequent reads enforce monotonicity.

### Crash recovery

`rela keys add` / `rela keys remove` walks the whole repo to re-seal
every data file under the new recipient set before updating
`recipients.age`. If rela crashes mid-walk, the next rela invocation
detects the in-flight rotation (via a sentinel file in the per-machine
state directory) and resumes it automatically. The rotation is
idempotent: files already migrated to the new version are skipped,
stragglers are re-sealed, `recipients.age` is rewritten, and the
sentinel is cleared. Nothing is required from the user.

## Quick start

### Enable encryption on a fresh project

```bash
# 1. Generate a keypair for yourself.
rela keys generate alice --out ~/rela-keys

# 2. Turn encryption on. The --identity flag copies your private key
#    to .rela/key so subsequent commands pick it up automatically.
rela keys init \
    --recipient alice \
    --pub-file ~/rela-keys/alice.pub \
    --identity ~/rela-keys/alice.key

# 3. Verify.
rela keys status
```

After `keys init`, every entity, relation, and attachment file is
sealed, and `<repo>/recipients.age` is the authoritative recipient
list. The operation refuses to run if the repo is already encrypted
or if it contains any file that's already sealed (half-migrated state).

When `--identity` is used, rela also appends `.rela/key` to the
project's `.gitignore` so the private key cannot be accidentally
committed.

### Check status

```bash
rela keys status
```

Reports whether the repo is encrypted and, if so, the recipient list
plus which recipient corresponds to the locally loaded identity
(marked `(you)`), along with the current version and repo UUID.

### Add a team member

```bash
rela keys add bob --pub-file bob.pub
```

The caller must be an existing recipient (have a working identity for
the current `recipients.age`). A new recipient cannot be added by
someone without a private key. Re-encrypts every data file and bumps
the version. After this command, bob gains access to **all existing
content** (cryptography cannot enforce forgetting old state).

### Remove a team member

```bash
rela keys remove bob
```

Re-encrypts every data file under the reduced recipient set and bumps
the version. Bob's identity is no longer a valid recipient going
forward. **This does NOT revoke access to any file bob already
decrypted and kept a copy of** — that's a fundamental property of any
at-rest encryption.

`keys remove` refuses to remove the last recipient, and refuses to
remove yourself (would lock you out). Use `keys decrypt` instead when
you want to go back to cleartext.

### Go back to cleartext

```bash
rela keys decrypt
```

Unseals every file and removes `recipients.age`.

## Recovery escape hatch

If rela itself is broken or you want to inspect a sealed file outside
the CLI, use the `age` binary directly. The private key lives in the
user-state directory — look up the exact path via the table in
"User-local state" above. On Linux:

```bash
KEY="$XDG_CONFIG_HOME/rela/repos/$(cat .rela/repo-id)/key"
age -d -i "$KEY" entities/requirements/REQ-001.md
```

The sealed plaintext begins with a small rela-specific header (one
line: `rela v=N path=...`) followed by the original entity bytes.
Pipe through `tail -n +2` to strip the header if you want just the
content.

## Threat model

**Protects against:**

- Casual disk read by someone who gets filesystem access.
- Accidental `git push` to a public remote.
- Stolen laptop without an unlocked private key.
- Read-only access to a CI runner cache.
- A cloud-storage provider (Dropbox, iCloud, etc.) reading the synced
  repo contents — **provided you do not sync `.rela/`** (see above).
- Adversary with storage write access who tries to silently add
  themselves as a recipient (requires decrypting the current
  `recipients.age`, which they can't).
- Adversary who rolls back a single sealed file to an older version
  (version-stamp check trips on read).
- Adversary who swaps one sealed file for another (path-stamp check
  trips on read).

**Does NOT protect against:**

- Recipients who decrypted past files keeping local copies.
- File deletion. If an adversary with storage write access deletes a
  sealed file, rela notices the entity is gone but cannot distinguish
  malicious deletion from legitimate deletion. Mitigation: cross-check
  against `git log` or run `rela analyze_orphans` / `rela
  analyze_cardinality` to surface missing references.
- Whole-repo rollback to before your last write — on a first-clone
  to a new machine there's no prior version state to compare against.
  Subsequent reads from the same machine do detect file-level
  rollback.
- Size correlation: file sizes are visible; very long ticket body =
  visibly longer sealed blob (age overhead is fixed).
- Modification-time correlation: `ls -la` reveals edit cadence.
- Leaks through the MCP server or Lua scripts: they see cleartext
  once loaded. Encryption is at-rest only.
- Malicious code running as your user: it already has your private
  key.

**Hard trade-offs:**

- No per-property or per-group encryption. A file is entirely sealed
  or entirely cleartext.
- Filenames under `entities/<type>/` and `relations/` are cleartext.
  Choose IDs and relation endpoints that don't leak information.

## Unsupported operations on encrypted repos

- **`rela rename-type`** — renaming an entity type on an encrypted
  repo is not supported. The operation currently reads files through
  the raw filesystem and would silently no-op on sealed files.
  Workaround: `rela keys decrypt` → `rela rename-type` → `rela keys
  init`. A follow-up release (pending a backend-layout refactor) will
  make this work transparently.

## Files on disk

```text
<repo>/
├── metamodel.yaml              cleartext (bootstrap)
├── recipients.age              sealed (authoritative recipient list)
├── .rela/
│   ├── key                     PRIVATE KEY — NEVER commit (gitignored, cleartext)
│   ├── fsstore-index.json      sealed (derived cache)
│   └── (other state)           cleartext, user-local — do not sync untrusted
├── entities/
│   └── requirement/
│       └── REQ-001.md          sealed
├── relations/
│   └── DEC-001--addresses--REQ-001.md   sealed
└── attachments/
    └── <hash-prefix>/
        ├── <hash>.<ext>        sealed (payload)
        └── <hash>.yaml         sealed (metadata sidecar)
```

Per-machine state (out of repo):

```text
$XDG_STATE_HOME/rela/repos/<repo-id>/
├── version                     highest sealed-file version observed
└── reseal-progress.yaml        present only during an interrupted rotation
```

## Reference

- Demo script: `demos/encryption/demo.sh` — exercises the full lifecycle
  end-to-end.
- CLI: [rela keys](cli-reference.md) — all subcommands.
- Decision record: `DEC-D5P4X` in the issues-and-design-tickets project.
- Security review: `.ignored/encryption-security-review.md` (project
  internal) — full findings, fixes, and known limitations.
