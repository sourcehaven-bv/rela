---
id: RR-G4A0Z1
type: review-response
title: HTTP tests use struct-literal metamodel, don't depend on loader fix
finding: 'newGlobalAttachmentsApp builds the Metamodel as a Go struct literal and injects via newAppFromParts, so Parse()/checkUnknownKeys is never called. Reverting the production whitelist fix leaves both HTTP tests (TestAttachmentUpload_GlobalAllowlistEnforced / GlobalScanCmdRejects) passing. Their doc comments claim ''without the loader fix such a metamodel can''t load at all'', which is true of a real deployment but false of the test. Fix: build the helper''s metamodel via metamodel.Parse([]byte(...)) so the integration tests also transitively depend on the whitelist entry (and the comment becomes accurate).'
severity: significant
resolution: 'Rewrote newGlobalAttachmentsApp to parse its metamodel from a real YAML string via metamodel.Parse (const globalAttachmentsMetamodelYAML) instead of a Go struct literal. The two HTTP integration tests now transitively depend on the whitelist fix. Verified: with the loader fix reverted, both tests fail at parse time with `unknown key "attachments"`. Updated the test doc comments to reflect the now-accurate linkage (fail at parse time, not just ''a real deployment'').'
status: addressed
---
