---
id: RR-DOCA05
type: review-response
title: identical_to comparing a request with itself was trivially true
finding: Nothing rejected other.Path == path with identical as/method/body, so api{path="/a", identical_to={path="/a"}} always passed — a claimless call wearing a claim's clothes, the exact class this feature exists to refuse.
severity: significant
resolution: A self-comparison is refused with a message explaining that the claim is about two DIFFERENT requests.
status: addressed
---
