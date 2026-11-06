---
id: RR-9T4G21Q
type: review-response
title: Flow version changes when keys are reordered
finding: The version hashes the comment-stripped YAML node. Reordering keys or states changes the hash although the flow means the same. A new version pins new runs to a new snapshot; old runs keep theirs.
severity: nit
reason: A reorder is an edit of the file. A spurious new version costs one extra stored snapshot and changes no run's behaviour. Canonicalising the node adds code for no user-visible gain.
status: wont-fix
---
