---
id: RR-UL121
type: review-response
title: 'Deferred: Lock file mode is 0o666 from lockedfile, not 0o600 like other userstate files'
finding: 'cranky-code-reviewer #11: lockedfile.Mutex creates the lock file at the OS umask, breaking the ''all files 0o600'' godoc claim.'
severity: nit
reason: Lock files contain no secrets; the 0o700 parent directory already restricts visibility. Chmodding after lockedfile creates the file would require cracking open the lockedfile API (it doesn't expose the file). Updated the docstring would be more honest — tracked as a docs-only follow-up.
status: deferred
---
