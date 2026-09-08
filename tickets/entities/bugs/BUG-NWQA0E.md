---
id: BUG-NWQA0E
type: bug
title: frontmatter.Split drops the trailing newline of a final block-scalar property
description: 'frontmatter.Split joins the frontmatter lines with "\n" and no terminator so a clip block scalar ("|") as the LAST property loses its final line break on read: fsstore writes "0\n" as "p: |\n    0\n" and reads back "0" while memstore / sqlitestore / pgstore keep "0\n". Silent and only for the last key. Found by the FuzzPropertyValuesTypeZoo round-trip assertion added for BUG-X7ICNM.'
priority: low
status: backlog
---

Fixed on branch fix/x7icnm-invalid-utf8 together with BUG-X7ICNM (one line in
frontmatter.Split plus a regression case in TestSplit), because the round-trip
assertion that found it could not ship red. Close this bug when that PR merges.
