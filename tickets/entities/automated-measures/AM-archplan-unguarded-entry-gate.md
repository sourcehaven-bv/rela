---
id: AM-archplan-unguarded-entry-gate
type: automated-measure
title: archplan fails CI on an HTTP entry point that writes without an authorization guard
description: Every data-entry entry point carries an //archplan:auth marker and every authorization check an //archplan:guard marker. archplan's unguarded_entry detector lists entry points that write without passing a guard. A planned archplan ci gate will fail the build on a new one. The detector exists; the CI gate does not yet.
kind: ci
location: '//archplan: markers in internal/dataentry; archplan unguarded_entry detector; archplan ci (planned)'
status: proposed
---
