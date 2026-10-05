---
id: RR-7XH3EG
type: review-response
title: Ref.IsZero doc does not match behaviour
finding: 'IsZero returns true for Ref{Face: draft}, which is not the zero value, while the doc said ''is the zero Ref''.'
severity: minor
resolution: 'Doc now says IsZero reports a Ref with no ID; test pins Ref{Face: draft}.IsZero() == true.'
status: addressed
---
