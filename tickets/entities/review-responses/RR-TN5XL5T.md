---
id: RR-TN5XL5T
type: review-response
title: Popup URL had no project prefix
finding: The popup opened at bare /quick so it relied on sync closing it on every switch
severity: nit
resolution: The popup now opens under /p/<id>/ of the project that owned the shortcut.
status: addressed
---
