---
id: RR-0Q2WP03
type: review-response
title: 'Design: un-reserving peer: makes it assertable'
finding: 'After the change a proxy header or JWT subject peer:acme is an ordinary user. An acl.yaml that granted peer:acme a role for #2156 would then grant it to whoever asserts that name.'
severity: minor
resolution: 'No release contains #2156 (git tag --contains 02ff7340a is empty), so no deployment holds such an assignment. The new decision records that peer: has no meaning and a sync client is named like any other principal.'
status: addressed
---
