---
id: RR-BGKBE50
type: review-response
title: go list errors are swallowed
finding: go list with stderr discarded drops a package with a syntax error, so the hook says no packages staged.
severity: minor
resolution: One go list -e call with a template that keeps packages with GoFiles, test files or InvalidGoFiles; only build-tag-only directories are skipped. Checked on a scratch module.
status: addressed
---
