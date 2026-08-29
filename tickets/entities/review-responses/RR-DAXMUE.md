---
id: RR-DAXMUE
type: review-response
title: Docs gave incomplete chmod advice — .rela is created 0755 and is not checked
finding: |-
    The permission check stats the file only. A 0600 secrets.yaml inside a 0755 .rela/ is reported clean — correct for reading, but project.Initialize (internal/project/context.go:142) creates .rela with MkdirAll(0755), so rela itself produces a world-readable, world-executable directory, and a group-writable parent would let another user replace secrets.yaml wholesale.

    On its own this would be out of scope for a file-mode check. It matters because the new docs tell operators `chmod 600 .rela/secrets.yaml` is the fix. Incomplete security advice is worse than none: the operator now believes they are done.
severity: minor
resolution: 'Fixed the docs rather than extending the check. docs/lua-scripting.md now shows both `chmod 700 .rela` and `chmod 600 .rela/secrets.yaml`, explains that rela init creates .rela as 0755 and that a group-writable parent allows outright replacement of the file, and states plainly that the check covers the file, not the directory. docs/mail.md carries the same correction in short form. Extending warnIfPermissive to the directory was considered and left out: .rela also holds non-secret cache and index files whose 0755 is not itself a fault, and tightening the mode is a change to what project.Initialize writes rather than what secrets reads — a different package and blast radius. The doc now describes the actual boundary instead of implying a wider one.'
status: addressed
---

## Resolution

Fixed the docs rather than extending the check.

`docs/lua-scripting.md` now shows both commands and says why:

```bash
chmod 700 .rela
chmod 600 .rela/secrets.yaml
```

with an explicit note that `rela init` creates `.rela/` as `0755`, that a
group-writable parent allows outright replacement of the file, and — stated
plainly so the guarantee is not overread — that **the check covers the file, not
the directory**. `docs/mail.md` carries the same correction in shorter form.

Extending `warnIfPermissive` to the directory was considered and left out. The
ticket's stated scope is the secrets file, `.rela/` also holds non-secret cache
and index files whose `0755` is not itself a fault, and tightening the directory
mode is a change to what `project.Initialize` *writes* rather than what
`secrets` *reads* — a different package and a different blast radius. The doc
now describes the actual boundary instead of implying a wider one.
