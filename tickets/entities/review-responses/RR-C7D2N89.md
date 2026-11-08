---
id: RR-C7D2N89
type: review-response
title: Test helper comments
finding: assertedRoles duplicates script.assertedRolesFrom and the DELETE stub handler looks unused.
severity: nit
resolution: Comments now say assertedRoles mirrors script.assertedRolesFrom (not importable) and that the DELETE route makes the mux answer 405 like Pratique.
status: addressed
---
