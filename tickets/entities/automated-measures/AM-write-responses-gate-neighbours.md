---
id: AM-write-responses-gate-neighbours
type: automated-measure
title: Write responses and entity ETags disclose nothing hidden
description: Create, attachment-upload and history-restore responses omit a neighbour the caller cannot read; entity ETags do not depend on a visible:-hidden value.
kind: test
location: internal/dataentry/write_response_neighbors_test.go (Test*_ResponseHidesUnreadableNeighbor); internal/dataentry/etag_redaction_test.go (TestEntityETag_ExcludesHiddenFields)
status: active
---
