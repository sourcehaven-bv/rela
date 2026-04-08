---
id: RR-ZHB6
type: review-response
title: filter_controls defaults vs URL filters not specified
finding: 'If filter_controls has a default (e.g. status=open) and URL says filter[status]=closed, which wins? Depends on FilterBar''s initializeFilters timing. Plan doesn''t say. Spec it: URL wins, FilterBar must not overwrite from defaults if URL value present.'
severity: minor
resolution: URL is read first into props, FilterBar's initializeFilters reads from props. Ordering ensures URL wins over filter_controls defaults. Specified in plan section 7.
status: addressed
---
