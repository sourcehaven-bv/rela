---
id: RR-BNZXD7
type: review-response
title: OpenAPI lifecycle paths omit 422 and 413
finding: internal/openapi/channels.go:193 lists 400/403/404/409/429. But `open` → resolveAnchor answers 422 face_required for a face-anchored type (the head GET lists 422), and decodeChannelBody answers 413 above channelBodyLimit. Add both codes, and pin them in channels_test.go:95.
severity: minor
resolution: The lifecycle paths now list 400, 403, 404, 409, 413, 422 and 429. TestChannelPaths pins 413 and 422 for close and delete.
status: addressed
---
