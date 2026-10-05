---
id: RR-1JGEEO
type: review-response
title: Upload suffixing confirms a hidden face's file names
finding: resolveAttachName suffixes against names on faces the uploader cannot read so the returned name confirms a guessed hidden file name
severity: minor
reason: Any deterministic collision rule over shared bytes signals the collision; the fix is per-face byte keys which is Stage 2 store API work. Exploiting it needs write on a face of the same entity and a guessed name. Recorded in design section 8.8.
status: deferred
---

**Where:** design section 4, "Upload".

`resolveAttachName` suffixes against `taken`, which includes names referenced by
faces the uploader cannot read. The returned name (`a.pdf` vs `a-1.pdf`) tells
the uploader whether a hidden face references `a.pdf`. That is a
guess-confirmation oracle on a hidden face's file names. It needs write on a
face of the same entity, and the attacker must guess the name.
