---
id: RR-W7MJRQ2
type: review-response
title: 'Code: picker reads owning edges without paging'
finding: 'If the relations endpoint paginated, owned targets past the first page would not be offered.'
severity: minor
reason: 'The /{plural}/{id}/relations/{rel} endpoint returns every edge; it has no paging.'
status: wont-fix
---
