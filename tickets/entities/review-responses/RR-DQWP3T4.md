---
id: RR-DQWP3T4
type: review-response
title: File metadata in Message.Props breaks on edit and can be forged
finding: 'Plan stored file entries in Message.Props: Store.Edit replaces props so files are lost or orphaned; JSON POST can forge entries; entries count against message_bytes; render depends on current config; scope contradicted itself on edit.'
severity: significant
resolution: Plan now stores a store-owned Message.Files field in the row JSON. Append sets it; Edit never touches it; Remove clears it. Normalize refuses client values for file properties and the JSON schema omits them. PredicateFields reads counts from Files. Edit keeps files.
status: addressed
---
