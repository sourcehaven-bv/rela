---
id: RR-5JDIQZ7
type: review-response
title: A missing commodity became a fake currency
finding: fmt.Sprint(nil) turned an unset commodity into '<nil>'.
severity: minor
resolution: checkBalance reads the commodity as a string and refuses a missing one.
status: addressed
---
