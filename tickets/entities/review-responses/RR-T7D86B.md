---
id: RR-T7D86B
type: review-response
title: rela-desktop comments call every single-process backend locked
finding: cmd/rela-desktop main.go and window.go say a single-process backend takes an exclusive lock at Open; only sqlite does. The binary is sqlite-tagged.
severity: significant
resolution: Reverted to the sqlite-build wording.
status: addressed
---
