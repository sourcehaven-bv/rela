---
id: RR-YPSGGL1
type: review-response
title: 'Design: a second statement per write doubles index work'
finding: Filling after every write doubles non-HOT tuple versions and index maintenance.
severity: significant
resolution: 'Plan changed: single-row writes set the columns in the write statement, hashed from unmarshalProps(marshalProps(p)). The trigger clears only when element_hash did not change; the rare canonical no-op change is refilled in the same transaction.'
status: addressed
---
