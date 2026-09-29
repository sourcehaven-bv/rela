---
id: RR-V8M81O
type: review-response
title: 'Nits: hardcoded lock class id, stale prose, map key, CLI prompt'
finding: facedeleterace_test.go hardcodes the family lock key; manager.go and runner.go comments cite the removed anyFaceOf and the old stamping site; the cascade dedup key uses fmt.Sprint(FamilyFaces); the CLI confirmation prompt did not name the face.
severity: nit
resolution: 'Exported FamilyAdvisoryLockKeyForTest via export_test.go; rewrote the stale comments; the CLI prompt names the face. fmt.Sprint stays: the faces are sorted and face names are identifiers, so the key is deterministic.'
status: addressed
---
