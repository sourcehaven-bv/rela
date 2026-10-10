---
id: RR-1BMUCV
type: review-response
title: Several 500s still log nothing
finding: comments_handler.go (list, get, save default) and feed_handler.go answered 500 with an empty detail and dropped err.
severity: significant
resolution: All six go through writeInternalError, which logs the cause.
status: addressed
---
