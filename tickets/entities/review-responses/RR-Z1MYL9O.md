---
id: RR-Z1MYL9O
type: review-response
title: 'A non-sequence automations: value is dropped silently'
finding: 'migration Detect fires only on a non-empty sequence, so automations: given as a mapping is dropped without an error; metamodel.Parse also ignores the key. Fix: Detect any non-null value that is not an empty sequence.'
severity: minor
resolution: 'Detect fires on any value except a missing key / null / an empty list; the converter refuses a non-list value with file and line; metamodel.Parse and Load refuse a schema that still declares automations: (LoadWithoutMigrationCheck still accepts it for rela migrate). TestAutomationsKey_ParseAndLoad replaces TestParse_IgnoresAutomationsKey.'
status: addressed
---
