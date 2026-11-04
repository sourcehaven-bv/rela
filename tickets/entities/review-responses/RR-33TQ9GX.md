---
id: RR-33TQ9GX
type: review-response
title: A closed event stream reports as a missing marker
finding: After ready a stream error or server close is not recorded so until waits and blames the marker.
severity: minor
resolution: streamEvents records why the stream closed and until fails at once with that reason.
status: addressed
---
