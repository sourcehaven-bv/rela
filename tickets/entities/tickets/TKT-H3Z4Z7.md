---
id: TKT-H3Z4Z7
type: ticket
title: Pin extractions/setup-just to a commit SHA
kind: chore
priority: low
effort: xs
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

## Description

The archplan job in ci.yml and archplan-weekly.yml use the third-party action
`extractions/setup-just@v3`, a mutable tag. Pin it to the commit of v3.1.0
(`f8a3cce218d9f83db3a2ecd90e41ac3de6cdfd9b`) and keep the version in a comment.
Dependabot (github-actions ecosystem) updates SHA pins with version comments, so
its config needs no change. GitHub-owned actions keep their tags, as is the repo
convention.

## Acceptance criteria

- Every `extractions/setup-just` use is pinned to a 40-character SHA with a
`# v3.1.0` comment.
- CI passes.
