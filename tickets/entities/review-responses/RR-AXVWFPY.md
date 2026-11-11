---
id: RR-AXVWFPY
type: review-response
title: SPA fetches every image preview eagerly
finding: Every image file in view was fetched at once, with no size bound.
severity: minor
resolution: Previews only for images up to 2 MiB, at most 4 fetches at once, queue dropped with the message. Tested in useChannelFiles.test.ts.
status: addressed
---
