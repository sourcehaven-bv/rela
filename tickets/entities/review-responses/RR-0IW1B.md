---
id: RR-0IW1B
type: review-response
title: 'Minor: ensureKeyGitignored was dead code that only tests called'
finding: 'cranky-code-reviewer #9: function and 50-line test still present; production callers all removed by Phase 3. Documents behavior that no longer exists.'
severity: minor
resolution: Deleted ensureKeyGitignored from internal/cli/keys.go and removed its TestEnsureKeyGitignored block from keys_test.go. Comments in keys_test.go's writeEncryptedRepo helper updated to reflect that it bypasses the loader's resolution path.
status: addressed
---
