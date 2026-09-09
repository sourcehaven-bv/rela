---
id: RR-XS1DMH
type: review-response
title: scripts/generate-test-data.sh embeds a stale copy of the template and still has the bug
finding: 'scripts/generate-test-data.sh:258-302 is a hand-maintained fork of DefaultMetamodelYAML() whose four entities (requirement, decision, solution, component) carry id_prefix but no id_type. It therefore still generates a project that fails to load, and the new regression test cannot see it because that test drives projectsetup.Initialize, not the shell script. This is the same duplicate-source-of-truth failure that allowed the original bug: the fix landed in one copy of the template.'
severity: critical
status: open
---
