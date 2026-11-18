---
id: TKT-EOFAO77
type: ticket
title: 'seqtrace: record the same-package functions between drawn calls'
kind: chore
priority: low
effort: s
status: done
---

## Description

seqtrace draws only calls that cross a package. A call made by a same-package
helper, or by a middleware around the handler, is drawn as a call of the
handler. Tools that read `steps.json`, such as archplan, then see a shared
helper as steps that every handler copied.

Add `in` to a step in `steps.json`: the functions between the call drawn above
and the call itself, outermost first. It is empty when the function drawn above
made the call. Diagram comparison ignores it.

## Acceptance criteria

1. A step whose call was made by a same-package helper lists that helper in `in`.
2. With a root label, the chain starts with the traced root function.
3. A step made by the function drawn above has no `in`.
4. `sameShape` and the comparison report ignore `in`.
