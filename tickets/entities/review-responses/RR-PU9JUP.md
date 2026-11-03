---
id: RR-PU9JUP
type: review-response
title: 'A null in: fails open'
finding: 'The plan uses In *[]string so that in: [] can be refused while an absent key means every state. yaml.v3 leaves the pointer nil for in: with no value and for in: ~, and keyChecker skips null scalars (internal/realtimecfg/keys.go, the !!null early return). So an operator who writes in: and forgets the list gets a type that is postable in every state, with no problem reported. This contradicts the security section''s claim that a mistake fails toward refusing. Detect the key in the yaml.Node (or decode into a custom type with an UnmarshalYAML that records presence) and refuse a null with "name the states, or drop in to allow every state". Add in: and in: ~ to the negative tests.'
severity: minor
resolution: 'Presence of `in` is read from the YAML node, so a bare `in:` or `in: ~` is refused with "name the states, or drop in to allow every state", and both are in the negative tests.'
status: addressed
---
