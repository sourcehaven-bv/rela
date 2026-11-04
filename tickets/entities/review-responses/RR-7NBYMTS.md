---
id: RR-7NBYMTS
type: review-response
title: A user pile named like a declared key is shadowed by name
finding: ByName and Push match a declared key before a name so a user pile named exactly 'favorites' is unreachable by that name.
severity: minor
reason: 'By design and documented: the key is matched before any name. The pile stays reachable by id and its name can be changed.'
status: wont-fix
---
