---
id: RR-BJDDA
type: review-response
title: status-bar git-status test has zero CI coverage
finding: Temp dir isn't a git repo, so isGitAvailable branches all pass vacuously. Either init git in test project or assert the hidden state explicitly.
severity: significant
reason: git-in-temp-project setup would require running `git init` in the fixture, setting up a branch, and then we'd have a repo in /tmp that accumulates locks. Not worth it for the narrow coverage gap. The test is explicit about being conditional.
status: deferred
---
