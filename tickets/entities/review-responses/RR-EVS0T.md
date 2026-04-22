---
id: RR-EVS0T
type: review-response
title: 250-line inline YAML has no schema validation
finding: Typo in a property name surfaces twenty tests in as cryptic 'element not visible'. Either extract to real YAML files or add a one-time smoke test that fetches _config to fail fast.
severity: significant
reason: Inline YAML string literals trade editor support for co-location. Extraction to real YAML files is a bigger refactor than this ticket's scope. The concrete failure mode (typo → cryptic test failure) is low-frequency; defer until the fixture actually needs editing.
status: deferred
---
