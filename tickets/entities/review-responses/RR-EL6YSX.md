---
id: RR-EL6YSX
type: review-response
title: Multi-type picker ships every search hit
finding: EntityPickerModal.vue:105-113 filters several allowed types in the browser after an unlimited /_search, so a broad query transfers every match to show a few. This is acceptable now. When it matters, let /_search take several `type` values instead of filtering client-side.
severity: nit
resolution: Client-side filtering after one search is acceptable at current sizes, as the finding says, and a multi-type /_search is a server API change outside this slice.
reason: Client-side filtering after one search is acceptable at current sizes, as the finding says, and a multi-type /_search is a server API change outside this slice.
status: wont-fix
---
