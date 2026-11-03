---
id: RR-O3HFWZ
type: review-response
title: Git status test loops without subtests and covers only the policy mode
finding: TestHandleGitStatus_CanSyncAndConflictFiles looped without t.Run, never asserted ConflictFiles at handler level despite its name, and skipped no-policy and read-only. (cranky-code-reviewer)
severity: nit
resolution: Renamed to TestHandleGitStatus_CanSync and made table-driven over no policy, read-only, policy without and with git:sync, with t.Run per case. ConflictFiles stays covered by TestGitStatusResponse_ConflictFiles.
status: addressed
---
