---
id: AM-sqlite-timestamp-order
type: automated-measure
title: sqlite stored timestamps sort in chronological order
description: A test that writes timestamps with trailing zeros in their fractional seconds and asserts the sweep's string comparisons order them chronologically.
kind: test
location: internal/store/sqlitestore (added in the BUG-HEIAVS fix PR)
status: proposed
---
