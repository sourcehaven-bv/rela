---
id: TKT-LDI9COS
type: ticket
title: 'e2e: buildIfMissing passes -o after the package path'
kind: chore
priority: low
effort: xs
status: backlog
---

## Description

`buildIfMissing` in `e2e/tests/fixtures.ts` runs `go build ./cmd/rela-server -o
<path>`. Go stops reading flags at the first package path, so `-o` is read as a
package and the build fails. Running the e2e suite locally without a prebuilt
`bin/rela-server` therefore fails. CI is not affected: it builds the binary
first.

Fix: pass `-o <path>` before the build args, as `["build", "-o", binaryPath,
...buildArgs]`.

Found while working on TKT-V05706X.
