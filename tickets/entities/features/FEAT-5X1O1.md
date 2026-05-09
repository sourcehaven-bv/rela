---
id: FEAT-5X1O1
type: feature
title: Graceful handling of git-crypt encrypted entity/relation files
summary: Detect git-crypt encrypted entity and relation files at the storage layer and surface them in the data-entry UI as inaccessible placeholders with actionable guidance, instead of failing or displaying raw ciphertext.
description: |-
    Some users keep parts of their rela project encrypted with git-crypt so that sensitive entities and relations are stored as ciphertext in the git repository. A user that has not unlocked the working tree (no key configured, or `git-crypt unlock` was never run) sees the encrypted blob on disk instead of a markdown file.

    Today this surfaces either as a parse error (markdown frontmatter parser fails on the binary header) or as garbled content displayed in the data-entry UI. Both are confusing.

    The feature: detect the git-crypt magic header (`\0GITCRYPT\0`) at the fsstore layer and propagate an 'encrypted / inaccessible' state to consumers. The data-entry UI shows a placeholder card/row marking the entity as inaccessible, with a help link explaining that the file is git-crypt encrypted and pointing the user at `git-crypt unlock`.
priority: medium
status: proposed
---
