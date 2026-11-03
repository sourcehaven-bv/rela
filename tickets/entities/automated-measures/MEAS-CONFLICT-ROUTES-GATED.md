---
id: MEAS-CONFLICT-ROUTES-GATED
type: automated-measure
title: Conflict routes require git:sync
description: TestV1Conflicts_GitSyncGate and the golden cases conflicts.*, conflict-hidden.* and conflict-resolve.* refuse every conflict route without git:sync.
kind: test
location: internal/dataentry/conflicts_api_test.go
status: active
---

TestV1Conflicts_GitSyncGate and the golden cases conflicts.*, conflict-hidden.*
and conflict-resolve.* refuse the conflict list, detail and resolve without
git:sync. Row-gating was not chosen (BUG-T6HNKS).
