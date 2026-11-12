---
id: AM-export-covers-every-project-store
type: automated-measure
title: Export round-trip covers every project data store
description: A test that lists the data stores a project keeps (graph, attachments, flow runs, and any later one) and asserts that an export followed by an import restores each of them. A new store fails the test until the export handles it.
kind: test
location: internal/appbuild (export round-trip test; to be written)
status: proposed
---

Proposed with BUG-EWG0R90, where the flow run store was left out of the export
because nothing listed the stores an export must cover.
