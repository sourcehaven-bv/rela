---
id: RR-1R82NF
type: review-response
title: Embedded EntityDetail navigates the whole app on a retired id
finding: The SPA redirect in loadView's prepare hook ran for every EntityDetail, including embedded ones (EntityPreviewModal, EntityDetailPanel, SidebarFlyout) that set followOwner=false. A preview of a retired id would replace the host page's route.
severity: significant
resolution: The redirect now runs only when props.followOwner is true. An embedded preview shows the successor in place. New test 'shows the successor in place in a preview' asserts no router.replace and that the successor renders.
status: addressed
---
