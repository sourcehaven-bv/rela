<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Encrypting a rela repo with git-crypt

rela itself stores everything on disk in cleartext — that is intentional:
your editor, `grep`, `diff`, merge tools, and any script you write all
work natively against the files. When the threat model is "the git
remote may leak or be compromised," push the encryption boundary out
to the git-sync path using [git-crypt](https://github.com/AGWA/git-crypt).

## When to use this

- You commit sensitive entity content and push to a git host whose
  operators you do not fully trust (public clouds, hosted-SCM SaaS,
  Dropbox-as-remote, etc.).
- You want multi-recipient access — each collaborator has their own
  GPG key, and losing any one person's key does not lock everyone
  else out.
- You still want `git` to work normally on local checkouts (commits,
  branches, merges, blame, log).

## What it protects

- Contents of every file configured through `.gitattributes` — entity
  markdown, relation markdown, attachments.
- Contents only. **Filenames, directory structure, commit messages,
  and commit metadata remain cleartext on the remote.** If an entity
  ID itself is sensitive, rename it before committing.

## What it does not protect

- `git log`, `git blame`, `git diff` on the remote show ciphertext
  bytes. PR reviews on GitHub etc. will be unreadable for encrypted
  files — merge conflicts too.
- Files committed before enabling git-crypt remain in history in the
  clear. Rewriting history with `git filter-repo` is possible but
  brittle; easier to treat the moment of enabling as a new epoch.

## One-time repo setup

```bash
# Install git-crypt (macOS)
brew install git-crypt

# Inside your rela project
cd my-project
git-crypt init

# Mark rela data files as encrypted
cat >> .gitattributes <<'EOF'
entities/** filter=git-crypt diff=git-crypt
relations/** filter=git-crypt diff=git-crypt
attachments/** filter=git-crypt diff=git-crypt
EOF

git add .gitattributes
git commit -m "chore: encrypt rela data files with git-crypt"

# Export your unlock key for safe offline storage
git-crypt export-key ~/secure-backup/rela-project.key
```

Your working tree stays cleartext. `git push` sends ciphertext to the
remote; `git clone && git-crypt unlock` gets you back to cleartext.

## Adding a collaborator

git-crypt supports two recipient modes. The GPG mode is the usual
pick for teams:

```bash
# Collaborator publishes their GPG key via any channel (key server,
# keybase, Signal). You import it:
gpg --import alice.pub

# Add them as an authorized recipient
git-crypt add-gpg-user alice@example.com

git push
```

Alice can then run `git-crypt unlock` after cloning without any extra
key file.

## Operational notes

- **Never commit `.git/git-crypt/` contents.** The default setup
  already `.gitignore`s it.
- **CI needs the key.** Export a symmetric key and store it as a CI
  secret; run `git-crypt unlock /path/to/key` early in the pipeline.
  Alternatively skip encrypted paths in CI if it does not need to
  read them.
- **Removing a collaborator** requires rewriting history (for
  forward secrecy) or rotating the symmetric key and re-encrypting
  the repo — same caveat as any at-rest encryption scheme. Removed
  users retain whatever they already decrypted locally.

## Why rela does not do this in-process

Earlier releases of rela included a first-party at-rest encryption
feature (`rela keys init`, `rela keys add`, etc.) that sealed every
file on disk. It was removed in favour of git-crypt for three
reasons:

1. **External tools stopped working.** `grep`, editor plugins, CI
   scripts, and merge tools could not read sealed files without going
   through rela.
2. **Merge conflict resolution was impossible on ciphertext.** Local
   edits sometimes required `rela keys decrypt` just to resolve a
   three-way merge.
3. **git-crypt already exists and is widely audited.** Rebuilding
   equivalent crypto in rela's process duplicated that work and
   created a second failure mode.

git-crypt puts confidentiality exactly where the threat lives — at
the sync boundary — while leaving the local working tree untouched.
