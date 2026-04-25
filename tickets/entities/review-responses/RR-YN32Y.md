---
id: RR-YN32Y
type: review-response
title: filepath.SkipDir from a file callback skips rest of directory
finding: 'loader_test.go: when walkErr != nil for a file entry (e.g., permission-denied symlink, mid-walk file deletion), the handler returns filepath.SkipDir. Per filepath.Walk docs, returning SkipDir from a *file* callback skips the remaining files in that file''s directory — so one transient FS hiccup mid-walk could silently drop every metamodel.yaml that comes alphabetically after it. Test would still pass on the metamodels already collected. Works on darwin today but is fragile. Fix: for a file walkErr, log and return nil (continue). Only return SkipDir when info.IsDir() == true. Also: info may itself be nil when walkErr != nil, so info.IsDir() must be guarded.'
severity: significant
resolution: Switched filepath.Walk to filepath.WalkDir (newer API, fewer Lstat calls). For walkErr on a file entry, log and return nil (continue) instead of SkipDir. Only return SkipDir when d.IsDir() is true. d is nil-guarded since the docs say it can be nil when walkErr is non-nil.
status: addressed
---
