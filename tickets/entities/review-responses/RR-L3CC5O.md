---
id: RR-L3CC5O
type: review-response
title: 'GetDefaultCloneDir discarded os.UserHomeDir error, yielding a relative base'
finding: |-
    GetDefaultCloneDir did `homeDir, _ := os.UserHomeDir()` then returned
    filepath.Join(homeDir, "rela-projects"). On failure homeDir is "", so it
    returned the RELATIVE path "rela-projects", which containedPath resolves against
    the process working directory.

    Containment still HOLDS -- it is a real base and the check passes -- which is
    what makes it easy to miss. The guard is satisfied while the clone lands
    somewhere the user never chose: precisely the outcome the doc comment on
    CloneOptions.BaseDir argues against when it rejects a CWD default as "a
    different surprise rather than a smaller one".
severity: significant
resolution: |-
    GetDefaultCloneDir now returns "" when there is no home directory to derive one
    from, and CloneProject refuses with a message asking the user to choose one
    explicitly rather than guessing.

    Consistent with the ticket's own reasoning: the answer to "no safe base" is to
    refuse, not to invent one. A discarded error had quietly reintroduced the
    alternative already considered and rejected.
status: addressed
---

## Resolution

GetDefaultCloneDir now returns "" when there is no home directory to derive one
from, and CloneProject refuses with a message asking the user to choose one
explicitly rather than guessing.

Consistent with the ticket's own reasoning: the answer to "no safe base" is to
refuse, not to invent one. A discarded error had quietly reintroduced the
alternative already considered and rejected.
