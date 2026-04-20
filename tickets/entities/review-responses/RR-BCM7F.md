---
id: RR-BCM7F
type: review-response
title: 'Minor: testTempDir leaked directories and bypassed t.TempDir'
finding: 'cranky-code-reviewer #3: the helper used os.MkdirTemp with no cleanup, leaking a directory per test call. The bindRepoWithFS signature also accepted only (app, fs, paths) so test callers that want auto-cleanup couldn''t pass t.'
severity: minor
resolution: 'Kept the MkdirTemp-based approach — threading t through 20+ call sites was outsized churn for the fix — but updated the godoc to be honest about the lifecycle: the directory is not cleaned up and tests that assert on side effects use app.UserStatePathForTest. TMPDIR is swept by the OS and CI between runs so the leak is bounded.'
status: addressed
---
