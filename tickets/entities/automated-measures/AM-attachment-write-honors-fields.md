---
id: AM-attachment-write-honors-fields
type: automated-measure
title: 'Test: attachment upload and delete honor the fields: policy'
description: 'Pins that an upload to or delete from a file field the fields: policy makes read-only is refused with 403 under the field rule and audited as an attachment write, and that a hidden field stays 404 (BUG-1N7OU9).'
kind: test
location: internal/dataentry/handlers_attachment_write_test.go
status: active
---
