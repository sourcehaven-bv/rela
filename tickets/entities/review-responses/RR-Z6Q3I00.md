---
id: RR-Z6Q3I00
type: review-response
title: Toggle errors name no key or line
finding: A malformed toggle failed config load with a message that did not say which key or line was wrong; yaml.v3 adds no position to a custom unmarshaler error.
severity: significant
resolution: ChromeToggle.UnmarshalYAML prefixes every error with the node line. Test case 'error names the line'.
status: addressed
---
