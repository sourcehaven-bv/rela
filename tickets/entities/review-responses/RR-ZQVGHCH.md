---
id: RR-ZQVGHCH
type: review-response
title: YAML parsed in full before the node cap
finding: yaml.v3 builds its whole tree before the budget runs.
severity: significant
resolution: decodeYAML counts line breaks, commas and brackets first and refuses input with more than the node cap (TestDecodeYAML_CountsBeforeParsing).
status: addressed
---
