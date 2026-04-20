---
id: RR-98TZZ
type: review-response
title: NewFS takes repoFp directly; force fingerprint resolution up the stack at every caller
finding: NewFS(repoFp) requires every caller to compute fingerprint before construction. Couples factory wiring to encryption state. Increases C1-style divergence risk.
severity: significant
resolution: 'Adopt leverage L1: NewFS(projectRoot string) resolves .rela/repo-id internally. Encrypted repos: service constructor cross-checks against Keyring.RepoID() and errors on mismatch (catches copied-in .rela/). Factory passes projectRoot (which it already has). Single place that knows the rule.'
status: addressed
---
