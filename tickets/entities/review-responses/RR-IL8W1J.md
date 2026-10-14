---
id: RR-IL8W1J
type: review-response
title: Symlink subtest passes on any error
finding: The sqlite escaping-symlink subtest only asserted err != nil.
severity: nit
resolution: It now asserts the error says 'cannot read module greeting'.
status: addressed
---
