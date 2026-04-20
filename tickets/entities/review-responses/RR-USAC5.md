---
id: RR-USAC5
type: review-response
title: 'Deferred: Put''s temp-name race and the absence of a shared AtomicWrite helper'
finding: 'cranky-code-reviewer #10 + #21: Put writes to a fixed `key + .tmp` filename, which races on concurrent Puts to the same key. Same pattern duplicated in keys.go writeAtomic and project.WriteRepoID.'
severity: significant
reason: 'Concrete impact is bounded: inter-process concurrent writes to ui-state.json and palette.yaml are rare in practice (single-user data-entry server typically), and the security-critical state (last_seen_version) is already guarded by lockedfile.Lock in StoreVersion. A proper internal/iox.AtomicWrite that refactors all three call sites is a bigger change than this PR should carry. Tracked as separate follow-up ticket.'
status: deferred
---
