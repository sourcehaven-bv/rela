---
id: RR-HLWRMK
type: review-response
title: 'Desktop cached the clone directory before Clone validated it'
finding: |-
    cmd/rela-desktop/main.go assigned d.lastCloneDir = targetDir BEFORE calling
    git.Clone. On a rejected traversal CloneProject returns an error, but
    lastCloneDir retains the unvalidated escaping path.

    That value is not inert: InitRelaProject reads it and calls os.MkdirAll on it.
    So containment stopped the clone while the path it rejected walked around the
    guard to reach a filesystem write anyway.
severity: significant
resolution: |-
    Moved the assignment below the successful Clone, with a comment recording why
    the order matters. A path that fails containment never enters app state.

    Worth noting the shape: the containment check itself was correct and complete.
    The problem was what the caller did with a value the check had already rejected.
    Validating at a boundary only helps if the rejected value stops there.
status: addressed
---

## Resolution

Moved the assignment below the successful Clone, with a comment recording why
the order matters. A path that fails containment never enters app state.

Worth noting the shape: the containment check itself was correct and complete.
The problem was what the caller did with a value the check had already rejected.
Validating at a boundary only helps if the rejected value stops there.
