---
id: TKT-IBT44K5
type: ticket
title: Make ygo upstream releases and advisories visible to the ygows fork
kind: chore
priority: high
effort: s
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

## Description

`third_party/ygows` forks the `provider/websocket` relay from
`github.com/reearth/ygo` v1.55.0 (TKT-6NPNITP). Dependabot bumps `reearth/ygo`
in `go.mod` and auto-merges on green CI, while the fork stays at v1.55.0. An
upstream release or an advisory on the relay is then invisible to the fork
(GitHub issue #2105, CISO finding 1 on PR #2103; POLICY-015 §5).

Make both visible:

- Record the forked upstream version in one const in the fork.
- A test fails when `go.mod`'s `reearth/ygo` version differs from it, telling the author to re-sync the fork. This blocks the Dependabot auto-merge.
- A scheduled workflow queries OSV for `github.com/reearth/ygo` advisories that touch `provider/websocket` and opens an issue.
- Document the process in `third_party/ygows/README.md` and the operator docs.
