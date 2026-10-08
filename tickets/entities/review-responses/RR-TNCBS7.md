---
id: RR-TNCBS7
type: review-response
title: Range starting inside emphasis or a link loses part of its first segment
finding: A segment starting inside **important** was closed by the parser at </strong>.
severity: significant
resolution: Segments lift a start out of any emphasis, link, image or strikethrough the segment runs past, to its opening delimiter (nested too). The SPA measures overlap by the range's extent including segments. Tests in quotefind and the golden fixture.
status: addressed
---
