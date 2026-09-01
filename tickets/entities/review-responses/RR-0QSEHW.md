---
id: RR-0QSEHW
type: review-response
title: Stale s.schemas reference in gitcrypt_integration_test.go comment after fileLayout extraction
finding: gitcrypt_integration_test.go:182 documented the scan-skip invariant as making buildInaccessibleEntity 'safe to assume entityType is always in s.schemas'. The schemas map moved to fileLayout, so s.schemas no longer exists — the only lingering reference to a removed field in the package. The equivalent prose in markdown.go was updated but this one was missed.
severity: minor
resolution: Updated the comment to 'in the layout's schemas', matching the wording already used in markdown.go. Verified with a fresh -count=1 run of ./internal/store/... plus comment-lint in the correct worktree.
status: addressed
---

Nit from the TKT-Y683LJ code review (cranky-code-reviewer, PR #1465). The
reviewer's verdict on the PR was sound/merge: tx.go byte-identical;
emit/notifyPut/notifyDelete/notifyRenamed bodies identical with lock-op count 66
in base and 66 on branch; the mu-held-across-batch watcher property untouched;
the purity claim structurally enforced by VALUE receivers (neither new type can
mutate) with zero references to the guarded state; exported-method set diffed
identical (33 vs 33, so the store.Store surface is unchanged); max-methods=81
verified by count; no test files changed.
