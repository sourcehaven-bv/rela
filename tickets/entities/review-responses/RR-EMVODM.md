---
id: RR-EMVODM
type: review-response
title: Unedited lines not byte-identical after re-encode
finding: 'yaml.v3 re-encode changes flow mappings in gantts: (data-entry.yaml:1453-1459) and drops blank lines.'
severity: significant
resolution: 'Already addressed in code: the edit is replayed as a three-way line merge over the original text (configedit/diff3.go); TestApply_UnchangedTreeIsByteIdentical covers both tickets files.'
status: addressed
---
