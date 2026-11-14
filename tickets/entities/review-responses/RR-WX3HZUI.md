---
id: RR-WX3HZUI
type: review-response
title: Redirect from the bare page loses present mode
finding: Opening /p/<page>?present=1 redirected to the first step without ?present=1.
severity: minor
resolution: The redirect keeps present and world on a steps page; unit test 'opens the first step of the bare page, presented if asked'.
status: addressed
---
