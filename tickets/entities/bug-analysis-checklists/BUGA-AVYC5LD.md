---
id: BUGA-AVYC5LD
type: bug-analysis-checklist
title: 'Analysis: Analyze reports hidden fields as missing'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestACLAnalyze_PropertiesSkipsHiddenFields fails on develop dc02375af: a hidden required field with a value is reported as "This field is required", and no finding names its field)
- [x] Minimal reproduction steps documented (a role whose `visible:` hides a required property; open the analysis page as that role; the Properties section reports the hidden field as missing)
- [x] Environment/conditions noted (any backend; seen in the harbourmaster customer portal, where customers do not see staff fields)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (skip a finding whose property is in the header's Redacted list, and prefix each property finding with the property label, or its name when it has none)
- [x] Regression test planned (TestACLAnalyze_PropertiesSkipsHiddenFields: one hidden required field with a value and one visible required field that is missing; only the visible one is reported, as "Summary: This field is required")
- [x] Related areas checked for similar issues (the Validations section runs custom rules on the raw store and only drops violators the reader cannot read; its messages come from the rule, so it is not affected in the same way; CLI and MCP analyze run without field redaction)
