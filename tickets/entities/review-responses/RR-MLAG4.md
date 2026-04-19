---
id: RR-MLAG4
type: review-response
title: Hybrid byte-format change bigger than 'prefix may differ'
finding: 'Hybrid recipients are ~1959 chars with prefix age1pq1... (vs ~62 chars age1...); identities AGE-SECRET-KEY-PQ-1... Hardcoded age1... and AGE-SECRET-KEY-1... strings in docs/cli-reference.md, docs-project/.../GUIDE-cli-reference.md, internal/cli/keys.go, docs/encryption.md, docs-project/.../GUIDE-encryption.md, internal/encryption/marshal.go, internal/encryption/identity.go, internal/store/fsstore/helpers_test.go. Plus identity_test.go leak detection asserts wrong prefix after flip. Also UX concern: --pub flag accepting a 1959-char pubkey on command line — should prefer --pub-file path.'
severity: significant
resolution: 'Every hardcoded age1.../AGE-SECRET-KEY-1... string enumerated by file:line in Part 2 ''Files affected''. Per user decision: drop --pub <string> CLI flag entirely and require --pub-file <path>. Pasting ~1959-char hybrid pubkeys on command line is not a UX worth defending.'
status: addressed
---
