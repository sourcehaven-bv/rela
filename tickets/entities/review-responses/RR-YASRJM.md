---
id: RR-YASRJM
type: review-response
title: List returned symlinks, crossing the containment boundary it inherits
finding: The entry filter was e.IsDir(), which is false for a symlink — including one pointing at a directory. List("scripts") therefore returned a symlink named evil.lua pointing at .rela/secrets.yaml as an ordinary config file. validateName guards the caller-supplied dir string and says nothing about entries. This seam is explicitly replacing os.OpenRoot call sites that DID defend against symlink escape (see the nested-root reasoning in dataentry/custom.go:117-125), so it inherits that duty.
severity: significant
resolution: 'Filter changed from !e.IsDir() to !e.Type().IsRegular(), so symlinks, devices and sockets are all skipped; the doc comment names .rela/secrets.yaml as the concrete target. Added TestFSLoader_List_SkipsSymlinks, which runs on a real filesystem (MemFS has no symlink support) and asserts a symlink pointing at a secret outside scripts/ is absent from the listing. Defence in depth rather than the only gate: Load still applies validateName, and a name returned by List is a bare filename — the value is that enumeration no longer advertises a path resolving outside the config tree.'
status: addressed
---

Filter changed from `!e.IsDir()` to `!e.Type().IsRegular()`, so symlinks,
devices and sockets are all skipped. Doc comment states why, naming
`.rela/secrets.yaml` as the concrete target.

Added `TestFSLoader_List_SkipsSymlinks`, which runs on a real filesystem (MemFS
has no symlink support) and asserts a symlink to a secret outside `scripts/` is
absent from the listing.

This is defence in depth rather than the only gate: `Load` still applies
`validateName`, and a name that came back from `List` is a bare filename. The
value is that enumeration no longer advertises a path that resolves outside the
config tree.
