---
id: RR-R6ZK1SK
type: review-response
title: Formatting-only body edit keeps the entity ETag
finding: Hashing the body through canonical (FormatMarkdown-normalized) means a reflow-only edit on pgstore or sqlitestore (which store the raw body) keeps the entity ETag. If-None-Match then answers 304 with stale bytes and If-Match passes against a different base.
severity: significant
resolution: The entity ETag folds the raw served body (length-prefixed) next to the view digest and the edges. A test pins that a formatting-only body change moves the entity ETag. The relation ETag keeps its pre-existing behaviour; see the deferred finding on it.
status: addressed
---
