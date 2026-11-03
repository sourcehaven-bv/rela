---
id: RR-EGV6J4
type: review-response
title: No read on the type returns 400 instead of an empty 200
finding: fieldfilter.go:104 returned false when no role reads the type, so a principal who can read no rows got 400 property_not_visible instead of an empty 200.
severity: minor
resolution: 'FieldVisibleOnEveryRow returns true when no role reads the type; pinned by TestFieldVisibleOnEveryRow_NoReaderPasses. The outsider and header-stranger goldens stay 400, because they are not in that branch: the fixture''s watches relation confers watcher, which reads ticket and hides secret behind when:, so they are in the same policy position as the viewer. Telling them apart would need the principal''s edges from the graph, which would make the refusal depend on rows. Their bodies now carry use and property.'
status: addressed
---
