---
id: RR-KOL12
type: review-response
title: Validator/automation/entitymanager paths block all writes when any encrypted file exists
finding: 'internal/validator/validator.go:159 aborts on first iterator error. After plan lands, ANY encrypted file in project causes validation pass to fail, which means data-entry PATCH cannot succeed (every write triggers validation). Same for automations that fan out reads on entity events. Plan''s ''failing loudly is acceptable for non-data-entry paths'' is wrong because validator IS on the data-entry write path. Decide: (a) validator skips-with-log encrypted entities (accepting false-negative cardinality on encrypted endpoints), or (b) data-entry handler hard-fails with explicit message ''validation skipped because project contains encrypted files''. Add AC and test.'
severity: significant
resolution: 'Resolved by validator change in AC9: rules whose target property is in e.Inaccessible are skipped with debug log, not error. The validator pass completes normally on projects containing encrypted files. The ''failing loudly'' framing in the original plan was wrong — partial encryption is the norm under git-crypt, so the validator must degrade gracefully or no data-entry write would ever succeed.'
status: addressed
---
