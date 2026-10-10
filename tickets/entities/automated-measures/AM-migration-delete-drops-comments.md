---
id: AM-migration-delete-drops-comments
type: automated-measure
title: Data-migration deletes notify comment cleanup
description: A datamigration test runs a face move and a delete step with comments enabled and asserts the affected threads are dropped or moved, never left at the old address.
kind: test
location: internal/datamigration/comments_test.go, internal/comments/service_test.go (TestFaceMoved), internal/cli/migrate_data_test.go (*CommentThreads), internal/appbuild/datamigration_test.go
status: active
---
