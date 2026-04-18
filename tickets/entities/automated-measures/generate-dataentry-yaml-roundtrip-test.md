---
id: generate-dataentry-yaml-roundtrip-test
type: automated-measure
title: 'Roundtrip test: generateDataEntryConfig output is valid YAML for all metamodels'
description: 'Feed generateDataEntryConfig a metamodel containing names with YAML-special chars (double quotes, backslashes, newlines in descriptions). Assert the returned string is valid YAML by round-tripping through yaml.Unmarshal and checking the decoded structure round-trips cleanly. This is the preventive measure for BUG-F9I2Z: any future change to the scaffold generator that re-introduces hand-built YAML will fail this test.'
kind: test
location: cmd/rela-desktop/main_test.go (TestGenerateDataEntryConfig_RoundTrip)
status: active
---
