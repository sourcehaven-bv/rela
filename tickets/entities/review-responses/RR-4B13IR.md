---
id: RR-4B13IR
type: review-response
title: Failed mapping check exits 0 without --expect
finding: acl map --webhook with a NOT HELD need exited 0; with --expect a matching file printed access matches and exited 1 without saying why.
severity: significant
resolution: 'A failed check exits 1 with or without --expect and prints ''Webhook <id>: mapping check failed.''. Docs updated. Tests added.'
status: addressed
---
