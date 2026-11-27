---
id: RR-O9AN177
type: review-response
title: Absence check proves less than its comment says
finding: 'TestWatcher_IgnoresNonMatchingExtensions: on macOS kqueue reports new files in directory-listing order, so a .txt event after the sentinel is only caught when it shares the sentinel''s debounce batch. The comment predates this diff.'
severity: minor
reason: Pre-existing comment; both creates come from one directory rescan in practice, so the check holds. Out of scope for a timeout fix.
status: wont-fix
---
