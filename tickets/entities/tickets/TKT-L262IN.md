---
id: TKT-L262IN
type: ticket
title: 'Triage osv-scanner findings from scheduled security scan (GH #1924)'
kind: chore
priority: medium
effort: xs
started: "2026-10-23"
completed: "2026-10-23"
status: done
---

## Problem

The scheduled security scan (GitHub #1924) reported two osv-scanner findings:

- GHSA-238p-pmpm-9mq7: npm `katex` 0.16.47 in `frontend/package-lock.json`,
fixed in 0.18.2. KaTeX treats inherited `Object.prototype` properties as
renderer options (prototype-pollution gadget). It comes in through `mermaid` 11
(`katex ^0.16.45`), which renders `$$...$$` math in diagram labels. Mermaid
11.17.2 still pins `^0.16.47`; only mermaid 12 (a major release with a new
default layout and theme) requires `^0.18.0`.
- GO-2026-5932: `golang.org/x/crypto/openpgp` is deprecated and unsafe. The
advisory covers every version, so no fix exists. rela does not import
`x/crypto/openpgp` under any build tag; go-git uses the ProtonMail fork.
govulncheck reports no reachable finding, so osv-scanner reports the module only
because it is in `go.mod`.

## Change

- Add an npm `overrides` entry that raises mermaid's `katex` to `^0.18.2`.
Mermaid only calls `katex.renderToString`, whose API is unchanged in 0.18.
Upgrading mermaid to 12 is out of scope.
- Add `osv-scanner.toml` with an `[[IgnoredVulns]]` entry for GO-2026-5932
and the reason. govulncheck needs no ignore, because it reports the advisory as
not called.

## Acceptance criteria

- osv-scanner reports no findings on the repository.
- Frontend unit tests, typecheck, lint and build pass.
- `npm ls katex` shows 0.18.x.

## Follow-up

Remove the `katex` override from `frontend/package.json` when rela moves to
mermaid 12, which requires `katex ^0.18.0` itself (RR-0WSPB4).
