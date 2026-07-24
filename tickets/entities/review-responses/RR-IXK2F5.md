---
id: RR-IXK2F5
type: review-response
title: Include root-only guard for `description` is untested
finding: 'include.go:137-139 rejects `description:` in included partial files (like version/namespace), but no test exercises it. include_test.go has TestLoadWithIncludes_IncludeHasVersion / _IncludeHasNamespace but no _IncludeHasDescription; docfields_test.go is parse-only. An untested guard rots: a future refactor could drop it and an included `description:` would be silently accepted + discarded. Fix: add the sibling test (5-line copy of the namespace one).'
severity: significant
resolution: 'Added TestLoadWithIncludes_IncludeHasDescription (include_test.go) — a partial file carrying `description:` is rejected with IncludeHasRootFieldError{Field: "description"}, mirroring the version/namespace sibling tests. The guard is now covered.'
status: addressed
---
