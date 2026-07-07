---
id: metamodel-toplevel-key-whitelist-parity-test
type: automated-measure
title: 'Test: metamodel top-level yaml keys stay in sync with validTopLevelKeys'
description: Regression + prevention for BUG-5XIN07. TestValidTopLevelKeysMatchStruct uses reflection to assert every top-level `yaml` tag on the metamodel.Metamodel struct is present in validTopLevelKeys, so a newly-added top-level struct field can never be silently rejected by the loader's checkUnknownKeys. TestParse_AttachmentsTopLevelKeyAccepted pins the specific `attachments:` case round-tripping through Parse(). Two data-entry HTTP tests (TestAttachmentUpload_GlobalAllowlistEnforced/GlobalScanCmdRejects) parse a real metamodel with a global attachments block through the loader, so they also fail at parse time if the whitelist entry is removed.
kind: test
location: internal/metamodel/loader_test.go (TestValidTopLevelKeysMatchStruct, TestParse_AttachmentsTopLevelKeyAccepted); internal/dataentry/handlers_attachment_write_test.go (TestAttachmentUpload_GlobalAllowlistEnforced, TestAttachmentUpload_GlobalScanCmdRejects)
status: active
---
