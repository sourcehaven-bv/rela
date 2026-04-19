---
id: GUIDE-encryption
type: guide
title: "At-Rest Encryption"
status: published
order: 11
audience: advanced
summary: "Encrypt entity, relation, and attachment files transparently using age"
---

rela can transparently seal every entity, relation, and attachment file in your
project so that the on-disk contents are unreadable without an authorized
private key. Encryption uses [age](https://age-encryption.org/) under the hood:
no custom crypto, one data key per file, X25519 recipients.

## When to use it

Turn encryption on when:

- The project contains sensitive design material and may end up on a stolen
  laptop, a shared filesystem, or a backup that escapes your control.
- You want to commit the repo to a remote (GitHub, GitLab) while keeping the
  contents private to a known set of team members.
- Your threat model is "adversary with disk read access but no running
  process and no private key." It is NOT a defense against a live attacker
  who can observe file sizes, modification times, or filename patterns.

Leave it off if the data is not sensitive or if you need `git diff`-style
readable history on the raw files.

## Mental model

| File                              | Sealed? | Notes                                       |
| --------------------------------- | ------- | ------------------------------------------- |
| `entities/<type>/*.md`            | yes     | Full file (frontmatter + body)              |
| `relations/*.md`                  | yes     | Relation filename stays cleartext           |
| `attachments/<id>/<prop>/<file>`  | yes     | Opaque binary sealed; filename cleartext    |
| `.rela/fsstore-index.json`        | yes     | Derived cache — would leak property values  |
| `metamodel.yaml`                  | no      | Bootstrap config; tooling needs it readable |
| `groups.yaml`, `schedules.yaml`   | no      | Bootstrap config                            |
| `templates/`                      | no      | Project scaffolding                         |
| `.rela/encryption.yaml`           | no      | Public recipient routing info               |
| `<repo>/keys/<name>.pub`          | no      | Public keys — by definition public          |

Filenames and file sizes are visible to anyone with disk read access. If a
filename would leak something sensitive (e.g., `REQ-project-terminus.md`),
rename before enabling encryption.

## Key management

Recipients are stored as age public keys in `<repo>/keys/<name>.pub`. The
filename stem is the recipient's human-facing name (`alice`, `bob`, etc.).
These `.pub` files are checked into git; they are public by design.

The local private identity is resolved in this order:

1. `$RELA_KEY_FILE` — explicit path via environment variable.
2. `<repo>/.rela/key` — per-repo identity (gitignored).
3. `~/.config/rela/key` — per-user identity shared across projects.

Any of these is an age private-key file in the standard
`AGE-SECRET-KEY-1...` format (single line). A missing identity at all three
paths is fine for read-only inspection of a cleartext repo; it becomes a
failure at decrypt time.

## Quick start

### Enable encryption on a fresh project

```bash
# 1. Generate a keypair for yourself.
rela keys generate alice --out ~/rela-keys

# 2. Turn encryption on. The --identity flag copies your private key
#    to .rela/key so subsequent commands pick it up automatically.
rela keys init \
    --recipient alice \
    --pub "$(cat ~/rela-keys/alice.pub)" \
    --identity ~/rela-keys/alice.key

# 3. Verify.
rela keys status
```

After `keys init`, every entity, relation, and attachment file starts with
the age header (`age-encryption.org/v1`). The operation refuses to run if
the repo is already encrypted, or if it contains any file that's already
sealed (half-migrated state).

When `--identity` is used, rela also appends `.rela/key` to the project's
`.gitignore` so the private key cannot be accidentally committed. Existing
`.gitignore` entries (including broader rules like `.rela/`) are preserved.

### Check status

```bash
rela keys status
```

Reports whether the repo is encrypted and, if so, the recipient list plus
which recipient corresponds to the locally loaded identity (marked `(you)`).

### Add a team member

```bash
rela keys add bob --pub "$(cat bob.pub)"
```

Re-encrypts every data file so bob can read. Bob's private key is on bob's
machine; only the public key travels. After this command, bob gains access
to **all existing content** (cryptography cannot enforce forgetting old
state), not just future writes.

### Remove a team member

```bash
rela keys remove bob
```

Generates a fresh effective recipient set (`alice` only) and re-encrypts
every data file under the reduced set. Bob's identity is no longer a valid
recipient going forward. **This does NOT revoke access to any file that
bob already decrypted and kept a copy of** — that's a fundamental property
of any at-rest encryption.

`keys remove` refuses to remove the last recipient. Use `keys decrypt`
instead when you want to go back to cleartext.

### Go back to cleartext

```bash
rela keys decrypt
```

Unseals every file, removes `.rela/encryption.yaml` and the recipient
pubkey files. Leaves any non-`*.pub` files in `keys/` alone (README,
user-organized subdirs, etc.).

## How operations are crash-safe

- **Individual writes** (entity create/update, relation add/remove,
  attachment upload) go through `temp-file + rename`. A crash leaves
  either the old file untouched or the new file fully sealed — never
  half-written plaintext.

- **`keys init`** walks the repo and writes each sealed file via
  `temp + rename`. A crash mid-walk leaves some files sealed and some
  still cleartext; `rela keys init` will refuse to re-run until the repo
  is consistent. Fix by removing `.rela/encryption.yaml` and re-running.

- **`keys add` / `keys remove`** is two-phase:
  1. Write every `<path>.rewrap.new` sealed under the new recipient set.
     No original files are touched.
  2. Rename each `.rewrap.new` → `<path>`. Individual renames are atomic
     on POSIX; the batch is not, but every path either holds the new
     sealed bytes or the old sealed bytes, never garbage.

  A crash between phase 1 and phase 2 leaves the old recipient set still
  readable and the `.rewrap.new` files as orphans (fsstore cleans up
  `.new`-suffixed files on open).

## Recovery escape hatch

If rela itself is broken or you want to inspect a sealed file outside the
CLI, use the `age` binary directly:

```bash
age -d -i ~/.config/rela/key entities/requirements/REQ-001.md
```

Same identity file, same wire format. No tooling dependency beyond the
age binary itself.

## Threat model

**Protects against:**

- Casual disk read by someone who gets filesystem access
- Accidental `git push` to a public remote
- Stolen laptop without an unlocked private key
- Read-only access to a CI runner cache

**Does NOT protect against:**

- Recipients who decrypted past files keeping local copies
- An attacker with write access to `<repo>/keys/` who substitutes
  pubkeys (v1 does not sign the recipient list)
- Size correlation: file sizes are visible; very long ticket body =
  visibly longer sealed blob (age overhead is fixed)
- Modification-time correlation: `ls -la` reveals edit cadence
- Leaks through the MCP server or Lua scripts: they see cleartext once
  loaded. Encryption is at-rest only.
- Malicious code running as your user: it already has your private key.

**Hard trade-offs:**

- No per-property or per-group encryption. A file is entirely sealed or
  entirely cleartext. This was a deliberate design choice: a prior
  per-group design shipped with a cross-property leakage bug.
- Filenames under `entities/<type>/` and `relations/` are cleartext.
  Choose IDs and relation endpoints that don't leak information.
- Post-quantum hybrid encryption is not implemented in v1. The age wire
  format supports recipient plugins; a ML-KEM-768 hybrid recipient is a
  planned follow-up.

## Files on disk

```text
<repo>/
├── metamodel.yaml              cleartext (bootstrap)
├── keys/
│   ├── alice.pub               cleartext (age public key, committed)
│   └── bob.pub                 cleartext (committed)
├── .rela/
│   ├── encryption.yaml         cleartext (marker + recipient list)
│   ├── key                     cleartext PRIVATE KEY — NEVER commit (gitignored)
│   └── fsstore-index.json      sealed   (derived cache)
├── entities/
│   └── requirement/
│       └── REQ-001.md          sealed
├── relations/
│   └── DEC-001--addresses--REQ-001.md   sealed
└── attachments/
    └── REQ-001/spec/
        └── spec.pdf            sealed
```

## Reference

- Demo script: `demos/encryption/demo.sh` — exercises the full lifecycle
  end-to-end with three recipients (alice, bob, eve).
- CLI: [rela keys](cli-reference.md) — all subcommands.
- Decision record: `DEC-D5P4X` in the issues-and-design-tickets project.
