---
id: RR-0L9DUD
type: review-response
title: mint took a bare string path, so the containment precondition was prose-only
finding: 'commandFileStore.mint documented that the caller must have already resolved the path through containedProjectPath, but the parameter was a plain string — every string in the package satisfied it, so the one call site that forgot would compile. Flagged by `just comment-report` as a param-contract finding: ''mint asserts a precondition the signature does not enforce — a named type would make the unvalidated call fail to compile.'''
severity: minor
resolution: 'Introduced containedPath, a struct whose only constructor is containProjectPath. mint now takes it, so an unchecked path is a compile error — verified with a throwaway probe file: ''cannot use "/etc/passwd" (untyped string constant) as containedPath value''. containedProjectPath stays string-returning for its other caller (resolveConflictPath in api_v1.go).'
status: addressed
---
