---
id: RR-HSAMKK3
type: review-response
title: Fake must return _actions
finding: getEntity and createEntity warn when _actions is missing, and EntityDetail gates controls on it. Ignoring _actions in the comparison would let the fake omit it.
severity: minor
resolution: The fixture has no acl.yaml, so the server's _actions is fixed; the fake returns it and the contract test compares it.
status: addressed
---
