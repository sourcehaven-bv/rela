---
id: RR-M90ZI
type: review-response
title: Part 2 first + no-back-compat breaks branch testers with cryptic error
finding: Plan claims 'no real users have X25519-sealed data, just re-run the demo.' But commit b6e50fd shipped rela keys CLI on this branch — anyone who pulled and ran 'rela keys generate' has X25519 keys on disk. After Part 2 lands, ReadIdentity (identity.go:113) does ids[0].(*age.X25519Identity) — type assertion fails for hybrid. User sees cryptic parse failure, not 'regenerate with new format' message.
severity: significant
reason: Feature hasn't been released — encryption-whole-repo branch is unmerged. Any X25519 keys on disk only exist in internal test repos; re-running rela keys generate is a trivial one-time cost for the <5 people affected. Not worth carrying dead-code detection for a format nobody will see post-merge. Branch testers who hit a cryptic parse error will understand the cause from commit history and regenerate.
status: wont-fix
---
